package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/event"
	"github.com/inno-agent/inno-agent/backend/pkg/telemetry"
)

type Result int

const (
	Done Result = iota
	Skip
	Transient
)

const (
	actionAssigned = "assigned"
	actionOpened   = "opened"
	actionEdited   = "edited"
)

const seenCap = 10_000

const (
	skipDecodeError         = "decode_error"
	skipNonIssueEvent       = "non_issue_event"
	skipUnhandledAction     = "unhandled_action"
	skipNotAssigned         = "not_assigned"
	skipMissingRef          = "missing_ref"
	skipDedup               = "dedup"
	skipNotOnboarded        = "not_onboarded"
	skipGenerationPermanent = "generation_permanent"
	skipPushPermanent       = "push_permanent"
	skipCommentPermanent    = "comment_permanent"
)

type boundedSet struct {
	m    map[string]struct{}
	keys []string
}

func newBoundedSet(cap int) *boundedSet {
	return &boundedSet{
		m:    make(map[string]struct{}, cap),
		keys: make([]string, 0, cap),
	}
}

func (s *boundedSet) has(key string) bool {
	_, ok := s.m[key]
	return ok
}

func (s *boundedSet) add(key string, cap int) {
	if _, ok := s.m[key]; ok {
		return
	}
	if len(s.keys) >= cap {
		oldest := s.keys[0]
		s.keys = s.keys[1:]
		delete(s.m, oldest)
	}
	s.m[key] = struct{}{}
	s.keys = append(s.keys, key)
}

type Processor struct {
	generator     domain.Generator
	pusher        domain.CodePusher
	prCreator     domain.PullRequestCreator
	poster        domain.CommentPoster
	logger        *zap.Logger
	botUsername   string
	onboardingURL string

	mu   sync.Mutex
	seen *boundedSet
}

func New(
	generator domain.Generator,
	pusher domain.CodePusher,
	prCreator domain.PullRequestCreator,
	poster domain.CommentPoster,
	logger *zap.Logger,
	botUsername, onboardingURL string,
) *Processor {
	return &Processor{
		generator:     generator,
		pusher:        pusher,
		prCreator:     prCreator,
		poster:        poster,
		logger:        logger.With(zap.String("layer", "processor")),
		botUsername:   botUsername,
		onboardingURL: onboardingURL,
		seen:          newBoundedSet(seenCap),
	}
}

func (p *Processor) Process(ctx context.Context, data []byte) (result Result) {
	start := time.Now()
	skipReason := ""
	telemetry.TrackConsumerInFlight(1)
	defer func() {
		telemetry.TrackConsumerInFlight(-1)
		telemetry.ObserveConsumerProcess(resultLabel(result), skipReason, time.Since(start))
		p.mu.Lock()
		telemetry.SetConsumerDedupSize(len(p.seen.keys))
		p.mu.Unlock()
	}()

	var env event.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		p.logger.Warn("undecodable envelope; skipping", zap.Error(err))
		skipReason = skipDecodeError
		return Skip
	}

	if !event.IsIssueEventType(env.EventType) {
		p.logger.Info(
			"non-issue event; skipping",
			zap.String("event_type", env.EventType),
			zap.String("delivery_id", env.DeliveryID),
		)
		skipReason = skipNonIssueEvent
		return Skip
	}

	issueEv, err := event.DecodeIssuePayload(env.Payload)
	if err != nil {
		p.logger.Warn("undecodable issue payload; skipping", zap.Error(err))
		skipReason = skipDecodeError
		return Skip
	}

	if issueEv.Action != actionAssigned && issueEv.Action != actionOpened && issueEv.Action != actionEdited {
		p.logger.Info(
			"issue action not handled; skipping",
			zap.String("action", issueEv.Action),
			zap.String("event_type", env.EventType),
			zap.String("delivery_id", env.DeliveryID),
		)
		skipReason = skipUnhandledAction
		return Skip
	}

	if !isAssignedToBot(issueEv, p.botUsername) {
		p.logger.Info(
			"issue is not assigned to the bot; skipping",
			zap.String("action", issueEv.Action),
			zap.String("bot_username", p.botUsername),
			zap.String("assignee", issueEv.Assignee.Name()),
			zap.String("issue_assignee", issueEv.Issue.Assignee.Name()),
			zap.String("delivery_id", env.DeliveryID),
		)
		skipReason = skipNotAssigned
		return Skip
	}

	assigner := issueEv.Sender.Name()
	ref := domain.IssueRef{
		Owner:         issueEv.RepoOwner(),
		Repo:          issueEv.RepoName(),
		Index:         issueEv.IssueIndex(),
		Assigner:      assigner,
		Title:         issueEv.Issue.Title,
		Body:          issueEv.IssueBody(),
		IssueType:     inferIssueType(issueEv.Issue.Labels),
		DefaultBranch: issueEv.Repository.DefaultBranch,
	}

	if ref.Owner == "" || ref.Repo == "" || ref.Index == 0 {
		p.logger.Warn(
			"issue payload missing repository or index; skipping",
			zap.String("owner", ref.Owner),
			zap.String("repo", ref.Repo),
			zap.Int64("index", ref.Index),
			zap.String("action", issueEv.Action),
		)
		skipReason = skipMissingRef
		return Skip
	}

	dedupKey := env.DeliveryID
	if dedupKey == "" {
		dedupKey = fmt.Sprintf("%s/%s/issue-%d@%s", ref.Owner, ref.Repo, ref.Index, issueEv.Action)
	}

	p.mu.Lock()
	already := p.seen.has(dedupKey)
	p.mu.Unlock()
	if already {
		p.logger.Info("dedup hit; skipping", zap.String("key", dedupKey))
		skipReason = skipDedup
		return Skip
	}

	issueLabel := fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Index)
	p.logger.Info("generating code for issue", zap.String("issue", issueLabel), zap.String("assigner", assigner))

	genResult, err := p.generator.Generate(ctx, ref)
	if err != nil {
		if errors.Is(err, domain.ErrNotOnboarded) {
			msg := fmt.Sprintf(
				"@%s, connect your account at %s to use the code generation bot.",
				assigner, p.onboardingURL,
			)
			if postErr := p.poster.PostIssueComment(ctx, ref, msg); postErr != nil {
				if errors.Is(postErr, domain.ErrTransient) {
					p.logger.Error("post not-onboarded comment transiently failed; will retry",
						zap.String("issue", issueLabel), zap.Error(postErr))
					return Transient
				}
				p.logger.Error("post not-onboarded comment permanently failed; skipping",
					zap.String("issue", issueLabel), zap.Error(postErr))
			}

			p.mu.Lock()
			p.seen.add(dedupKey, seenCap)
			p.mu.Unlock()
			skipReason = skipNotOnboarded
			telemetry.IncConsumerOutcome("not_onboarded")
			return Skip
		}

		if errors.Is(err, domain.ErrPermanent) {
			p.logger.Error("generation permanently failed; skipping message",
				zap.String("issue", issueLabel), zap.Error(err))
			skipReason = skipGenerationPermanent
			telemetry.IncError("issue-consumer", "generation_permanent")
			return Skip
		}

		p.logger.Error("generation failed; will retry", zap.Error(err))
		telemetry.IncError("issue-consumer", "generation_transient")
		return Transient
	}

	branch := fmt.Sprintf("innoagent-issue-%d", ref.Index)
	commitMsg := fmt.Sprintf("feat: implement issue #%d\n\nAuto-generated by InnoAgent", ref.Index)
	if genResult.Summary != "" {
		commitMsg = fmt.Sprintf("%s\n\n%s", commitMsg, genResult.Summary)
	}

	if err := p.pusher.PushFiles(ctx, ref, branch, genResult.Files, commitMsg); err != nil {
		if errors.Is(err, domain.ErrPermanent) {
			p.logger.Error("push permanently failed; skipping message",
				zap.String("issue", issueLabel), zap.Error(err))
			skipReason = skipPushPermanent
			telemetry.IncError("issue-consumer", "push_permanent")
			return Skip
		}
		p.logger.Error("push failed; will retry", zap.Error(err))
		telemetry.IncError("issue-consumer", "push_transient")
		return Transient
	}
	telemetry.IncConsumerOutcome("branch_pushed")

	prTitle := fmt.Sprintf("Implement issue #%d: %s", ref.Index, ref.Title)
	prBody := fmt.Sprintf("Auto-generated by InnoAgent.\n\nCloses #%d", ref.Index)
	if genResult.Summary != "" {
		prBody = fmt.Sprintf("%s\n\n%s", prBody, genResult.Summary)
	}

	reviewer := assigner

	var prIndex int64
	if p.prCreator != nil {
		prIndex, err = p.prCreator.CreatePullRequest(ctx, ref, branch, prTitle, prBody, []string{reviewer})
		if err != nil {
			if errors.Is(err, domain.ErrPermanent) {
				p.logger.Warn("pull request creation permanently failed; continuing with branch only",
					zap.String("issue", issueLabel), zap.Error(err))
				telemetry.IncError("issue-consumer", "pr_permanent")
			} else {
				p.logger.Error("pull request creation failed; will retry", zap.Error(err))
				telemetry.IncError("issue-consumer", "pr_transient")
				return Transient
			}
		} else if prIndex > 0 {
			telemetry.IncConsumerOutcome("pr_created")
		}
	}

	comment := buildSuccessComment(branch, prIndex, reviewer, genResult)
	if err := p.poster.PostIssueComment(ctx, ref, comment); err != nil {
		if errors.Is(err, domain.ErrPermanent) {
			p.logger.Error("post comment permanently failed; skipping message",
				zap.String("issue", issueLabel), zap.Error(err))
			skipReason = skipCommentPermanent
			telemetry.IncError("issue-consumer", "comment_permanent")
			return Skip
		}
		p.logger.Error("post comment failed; will retry", zap.Error(err))
		telemetry.IncError("issue-consumer", "comment_transient")
		return Transient
	}
	telemetry.IncConsumerOutcome("comment_posted")

	p.mu.Lock()
	p.seen.add(dedupKey, seenCap)
	p.mu.Unlock()

	p.logger.Info(
		"code pushed and comment posted",
		zap.String("issue", issueLabel),
		zap.String("branch", branch),
		zap.Int64("pull_request", prIndex),
	)
	return Done
}

func resultLabel(r Result) string {
	switch r {
	case Done:
		return "done"
	case Skip:
		return "skip"
	case Transient:
		return "transient"
	default:
		return "unknown"
	}
}

func isAssignedToBot(ev event.IssueEvent, botUsername string) bool {
	if botUsername == "" {
		return false
	}
	if strings.EqualFold(ev.Assignee.Name(), botUsername) {
		return true
	}
	if strings.EqualFold(ev.Issue.Assignee.Name(), botUsername) {
		return true
	}
	for _, a := range ev.Issue.Assignees {
		if strings.EqualFold(a.Name(), botUsername) {
			return true
		}
	}
	return false
}

func inferIssueType(labels []event.Label) string {
	for _, l := range labels {
		if strings.EqualFold(l.Name, "bug") {
			return "bug"
		}
	}
	return "issue"
}

func buildSuccessComment(branch string, prIndex int64, reviewer string, result *domain.GenerationResult) string {
	var sb strings.Builder
	sb.WriteString("Code generated and pushed to branch `")
	sb.WriteString(branch)
	sb.WriteString("`.\n\n")
	if prIndex > 0 {
		sb.WriteString(fmt.Sprintf("Pull request #%d opened", prIndex))
		if reviewer != "" {
			sb.WriteString(fmt.Sprintf(" with @%s as reviewer", reviewer))
		}
		sb.WriteString(".\n\n")
	}
	if result.Summary != "" {
		sb.WriteString("**Summary:** ")
		sb.WriteString(result.Summary)
		sb.WriteString("\n\n")
	}
	sb.WriteString("**Files changed:**\n")
	for _, f := range result.Files {
		sb.WriteString("- `")
		sb.WriteString(f.Path)
		sb.WriteString("`\n")
	}
	return sb.String()
}
