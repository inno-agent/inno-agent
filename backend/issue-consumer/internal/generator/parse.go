package generator

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

const codegenSystemPrompt = `You are a senior software engineer implementing GitFlame issues.

Return ONLY a single JSON object. No markdown, no code fences, no explanation.

Schema:
{"summary":"what you implemented","files":[{"path":"relative/path","content_base64":"BASE64_UTF8"}]}

Example for a Python script in main.py:
{"summary":"add two numbers","files":[{"path":"main.py","content_base64":"YSA9IGludChpbnB1dCgpCmIgPSBpbnQoaW5wdXQoKSkKcHJpbnQoYSArIGIp"}]}

Rules:
- content_base64 must be standard base64 of the full UTF-8 file (one line, no spaces). Do not gzip or compress.
- Include complete files, not diffs.
- Minimal changes only.`

const codegenRepairPrompt = `Your reply was not usable. Return ONLY one JSON object.
Do not use markdown fences or comments. Every file must have path and content_base64.
content_base64 must be valid standard base64 of raw UTF-8 file bytes (not gzip).
Example: {"summary":"done","files":[{"path":"main.py","content_base64":"cHJpbnQoMSk="}]}`

var (
	jsonFenceRE     = regexp.MustCompile("(?s)```(?:json)?\\s*([\\s\\S]*?)```")
	anyFenceRE      = regexp.MustCompile("(?s)```([a-zA-Z0-9._-]*)\\s*\\n([\\s\\S]*?)```")
	trailingCommaRE = regexp.MustCompile(`,(\s*[}\]])`)
)

type llmOutput struct {
	Summary string    `json:"summary"`
	Files   []llmFile `json:"files"`
}

type llmFile struct {
	Path          string `json:"path"`
	Content       string `json:"content"`
	ContentBase64 string `json:"content_base64"`
}

type fencedBlock struct {
	Lang    string
	Content string
}

func parseLLMOutput(raw string) (*domain.GenerationResult, error) {
	candidates := jsonCandidates(raw)
	var lastErr error
	for _, jsonText := range candidates {
		result, err := parseJSONObject(jsonText, raw)
		if err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}

	if result, err := parseFromMarkdownFences(raw); err == nil {
		return result, nil
	} else if lastErr == nil {
		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no parseable output found")
	}
	if strings.Contains(lastErr.Error(), "no code in markdown fences") {
		lastErr = fmt.Errorf("json found but file contents were missing or invalid base64")
	}
	return nil, fmt.Errorf("invalid json: %w", lastErr)
}

func parseJSONObject(jsonText, raw string) (*domain.GenerationResult, error) {
	jsonText = sanitizeJSONObject(jsonText)

	var out llmOutput
	if err := json.Unmarshal([]byte(jsonText), &out); err != nil {
		return nil, err
	}

	files, err := filesFromOutput(out, raw)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no files in output")
	}

	return &domain.GenerationResult{
		Files:   files,
		Summary: strings.TrimSpace(out.Summary),
	}, nil
}

func jsonCandidates(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	ordered := make([]string, 0, 8)
	seen := make(map[string]struct{})
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		ordered = append(ordered, s)

		if cleaned := sanitizeJSONObject(s); cleaned != s {
			if _, ok := seen[cleaned]; !ok && cleaned != "" {
				seen[cleaned] = struct{}{}
				ordered = append(ordered, cleaned)
			}
		}
	}

	add(trimmed)
	if obj := extractJSONObject(trimmed); obj != "" {
		add(obj)
	}

	for _, m := range jsonFenceRE.FindAllStringSubmatch(trimmed, -1) {
		if len(m) == 2 {
			add(m[1])
			if obj := extractJSONObject(m[1]); obj != "" {
				add(obj)
			}
		}
	}

	for _, m := range anyFenceRE.FindAllStringSubmatch(trimmed, -1) {
		if len(m) != 3 {
			continue
		}
		body := strings.TrimSpace(m[2])
		add(body)
		if obj := extractJSONObject(body); obj != "" {
			add(obj)
		}
	}

	return ordered
}

func extractJSONObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

func sanitizeJSONObject(s string) string {
	s = stripJSONComments(s)
	s = trailingCommaRE.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

func stripJSONComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	inString := false
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]

		if inString {
			b.WriteByte(c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			b.WriteByte(c)
			continue
		}

		if c == '/' && i+1 < len(s) {
			switch s[i+1] {
			case '/':
				i += 2
				for i < len(s) && s[i] != '\n' {
					i++
				}
				if i < len(s) {
					b.WriteByte('\n')
				}
				continue
			case '*':
				i += 2
				for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
					i++
				}
				if i+1 < len(s) {
					i++
				}
				continue
			}
		}

		b.WriteByte(c)
	}

	return b.String()
}

func filesFromOutput(out llmOutput, raw string) ([]domain.GeneratedFile, error) {
	fences := extractFencedBlocks(raw)
	files := make([]domain.GeneratedFile, 0, len(out.Files))
	fenceIdx := 0

	for _, f := range out.Files {
		path := strings.TrimSpace(f.Path)
		if path == "" {
			continue
		}

		content, err := decodeFileContent(f)
		if err != nil || content == "" {
			if fenceIdx < len(fences) && !looksLikeJSON(fences[fenceIdx].Content) {
				content = strings.TrimSpace(fences[fenceIdx].Content)
				fenceIdx++
			}
		}
		if content == "" {
			continue
		}

		files = append(files, domain.GeneratedFile{
			Path:    path,
			Content: content,
		})
	}

	if len(files) == 0 && len(fences) > 0 {
		return filesFromFences(fences)
	}

	return files, nil
}

func decodeFileContent(f llmFile) (string, error) {
	if enc := strings.TrimSpace(f.ContentBase64); enc != "" {
		if decoded, err := decodeBase64Lenient(enc); err == nil {
			if text, err := decodeFileBytes(decoded); err == nil && text != "" {
				return text, nil
			}
		}
	}
	if content := strings.TrimSpace(f.Content); content != "" {
		return content, nil
	}
	return "", fmt.Errorf("file %s has no content", f.Path)
}

func decodeFileBytes(raw []byte) (string, error) {
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return "", err
		}
		defer gr.Close()
		decompressed, err := io.ReadAll(gr)
		if err != nil {
			return "", err
		}
		raw = decompressed
	}
	return string(raw), nil
}

func decodeBase64Lenient(enc string) ([]byte, error) {
	enc = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, enc)
	if enc == "" {
		return nil, fmt.Errorf("empty base64")
	}

	for pad := 0; pad < 4; pad++ {
		candidate := enc + strings.Repeat("=", pad)
		if decoded, err := base64.StdEncoding.DecodeString(candidate); err == nil {
			return decoded, nil
		}
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(enc); err == nil {
		return decoded, nil
	}
	return nil, fmt.Errorf("invalid base64")
}

func parseFromMarkdownFences(raw string) (*domain.GenerationResult, error) {
	fences := extractFencedBlocks(raw)
	if len(fences) == 0 {
		return nil, fmt.Errorf("no markdown fences found")
	}

	files, err := filesFromFences(fences)
	if err != nil {
		return nil, err
	}

	return &domain.GenerationResult{
		Files:   files,
		Summary: "Generated from markdown code block",
	}, nil
}

func filesFromFences(fences []fencedBlock) ([]domain.GeneratedFile, error) {
	files := make([]domain.GeneratedFile, 0, len(fences))
	usedPaths := make(map[string]int)

	for i, fb := range fences {
		content := strings.TrimSpace(fb.Content)
		if content == "" || looksLikeJSON(content) {
			continue
		}

		path := defaultPathForLang(fb.Lang, i)
		if n := usedPaths[path]; n > 0 {
			ext := pathExtension(path)
			base := strings.TrimSuffix(path, ext)
			path = fmt.Sprintf("%s_%d%s", base, n+1, ext)
		}
		usedPaths[path]++

		files = append(files, domain.GeneratedFile{
			Path:    path,
			Content: content,
		})
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no code in markdown fences")
	}
	return files, nil
}

func extractFencedBlocks(raw string) []fencedBlock {
	var blocks []fencedBlock
	for _, m := range anyFenceRE.FindAllStringSubmatch(raw, -1) {
		if len(m) != 3 {
			continue
		}
		blocks = append(blocks, fencedBlock{
			Lang:    strings.ToLower(strings.TrimSpace(m[1])),
			Content: m[2],
		})
	}
	return blocks
}

func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

func defaultPathForLang(lang string, index int) string {
	switch lang {
	case "python", "py":
		if index == 0 {
			return "main.py"
		}
		return fmt.Sprintf("main_%d.py", index+1)
	case "go", "golang":
		return "main.go"
	case "javascript", "js":
		return "index.js"
	case "typescript", "ts":
		return "index.ts"
	case "java":
		return "Main.java"
	case "rust", "rs":
		return "main.rs"
	case "sh", "bash":
		return "script.sh"
	default:
		if index == 0 {
			return "solution.txt"
		}
		return fmt.Sprintf("solution_%d.txt", index+1)
	}
}

func pathExtension(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return path[i:]
	}
	return ""
}
