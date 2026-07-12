package generator

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseLLMOutput_RawJSON(t *testing.T) {
	raw := `{"summary":"done","files":[{"path":"a.go","content":"package main"}]}`
	result, err := parseLLMOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Path != "a.go" {
		t.Fatalf("unexpected files: %+v", result.Files)
	}
}

func TestParseLLMOutput_JSONFence(t *testing.T) {
	raw := "Here is the result:\n```json\n{\"summary\":\"ok\",\"files\":[{\"path\":\"b.go\",\"content\":\"x\"}]}\n```"
	result, err := parseLLMOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Path != "b.go" {
		t.Fatalf("unexpected files: %+v", result.Files)
	}
}

func TestParseLLMOutput_Base64Content(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("print('hi')\n"))
	raw := `{"summary":"ok","files":[{"path":"main.py","content_base64":"` + encoded + `"}]}`
	result, err := parseLLMOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files[0].Content != "print('hi')\n" {
		t.Fatalf("got %q", result.Files[0].Content)
	}
}

func TestParseLLMOutput_ExtractFromProse(t *testing.T) {
	raw := `Sure! Here you go:
{"summary":"ok","files":[{"path":"main.py","content_base64":"cHJpbnQoJ2hpJyk="}]}
Hope this helps.`
	result, err := parseLLMOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("unexpected files: %+v", result.Files)
	}
}

func TestParseLLMOutput_BrokenMultilineContentFailsJSON(t *testing.T) {
	raw := `{"summary":"add two numbers","files":[{"path":"add.py","content":"a = int(input())
b = int(input())
print(a + b)"}]}`
	_, err := parseLLMOutput(raw)
	if err == nil {
		t.Fatal("expected parse error for invalid multiline JSON content")
	}
}

func TestParseLLMOutput_PythonFenceOnly(t *testing.T) {
	raw := "Here is the solution:\n```python\na = int(input())\nb = int(input())\nprint(a + b)\n```"
	result, err := parseLLMOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Path != "main.py" {
		t.Fatalf("unexpected files: %+v", result.Files)
	}
	if !strings.Contains(result.Files[0].Content, "print(a + b)") {
		t.Fatalf("unexpected content: %q", result.Files[0].Content)
	}
}

func TestParseLLMOutput_JSONPathWithPythonFence(t *testing.T) {
	raw := `{"summary":"done","files":[{"path":"add.py"}]}
` + "```python\nprint(1)\n```"
	result, err := parseLLMOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files[0].Path != "add.py" || result.Files[0].Content != "print(1)" {
		t.Fatalf("unexpected: %+v", result.Files[0])
	}
}

func TestParseLLMOutput_MalformedBase64FallsBackToFence(t *testing.T) {
	raw := "```json\n{\"summary\":\"ok\",\"files\":[{\"path\":\"main.py\",\"content_base64\":\"not!!!valid\"}]}\n```\n```python\nprint(1)\n```"
	result, err := parseLLMOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files[0].Content != "print(1)" {
		t.Fatalf("got %q", result.Files[0].Content)
	}
}

func TestDecodeBase64Lenient_WithPadding(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("a=1"))
	encoded = strings.TrimRight(encoded, "=")
	decoded, err := decodeBase64Lenient(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "a=1" {
		t.Fatalf("got %q", decoded)
	}
}

func TestParseLLMOutput_ProseOnlyFails(t *testing.T) {
	_, err := parseLLMOutput("yes, here is a python script that adds numbers...")
	if err == nil {
		t.Fatal("expected parse error for prose-only response")
	}
}

func TestParseLLMOutput_JSONFenceWithGzipBase64(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write([]byte("# readme\n"))
	_ = gw.Close()
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	raw := "```json\n{\"summary\":\"ok\",\"files\":[{\"path\":\"README.md\",\"content_base64\":\"" + encoded + "\"}]}\n```"
	result, err := parseLLMOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files[0].Path != "README.md" || !strings.Contains(result.Files[0].Content, "# readme") {
		t.Fatalf("unexpected: %+v", result.Files[0])
	}
}

func TestParseLLMOutput_JSONFenceInvalidBase64FailsClearly(t *testing.T) {
	raw := "```json\n{\"summary\":\"ok\",\"files\":[{\"path\":\"README.md\",\"content_base64\":\"H4sIAAAAAAAAAAAAD2Rlc3QgAAAAA\"}]}\n```"
	_, err := parseLLMOutput(raw)
	if err == nil {
		t.Fatal("expected parse error for invalid file payload")
	}
	if !strings.Contains(err.Error(), "invalid base64") && !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseLLMOutput_JSONWithLineComments(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("# readme\n"))
	raw := "```json\n{\n  \"summary\": \"read markdown file\",\n  \"files\": [\n    {\n      \"path\": \"README.md\",\n      \"content_base64\": \"" + encoded + "\" // base64 of readme\n    }\n  ]\n}\n```"
	result, err := parseLLMOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files[0].Path != "README.md" || !strings.Contains(result.Files[0].Content, "# readme") {
		t.Fatalf("unexpected: %+v", result.Files[0])
	}
}

func TestStripJSONComments_PreservesSlashesInStrings(t *testing.T) {
	in := `{"url":"http://example.com","note":"keep // this"}`
	out := stripJSONComments(in)
	if out != in {
		t.Fatalf("got %q", out)
	}
}
