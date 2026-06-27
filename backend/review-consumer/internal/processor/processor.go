package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/event"
)

type Result int

const (
	Done      Result = iota // processed; commit offset
	Skip                    // irrelevant, dedup, or permanent error; commit offset
	Transient               // retriable failure; do NOT commit
)

// actionReviewerAdded is the GitFlame payload action that fires when a reviewer
// is added to a PR — the trigger for the bot. (The X-GitFlame-Event header is
// "pull_request"; the body action is the reliable discriminator.)
const actionReviewerAdded = "reviewer_added"

// seenCap is the maximum number of entries in the dedup set. When the cap is
// reached the oldest entry is evicted (FIFO) so the set stays bounded.
const seenCap = 10_000

// boundedSet is a simple capped dedup set.
// It is NOT safe for concurrent use on its own; callers hold p.mu.
type boundedSet struct {
	m    map[string]struct{}
	keys []string // ring-style FIFO of insertion order
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
	reviewer      domain.Reviewer
	poster        domain.CommentPoster
	logger        *zap.Logger
	botUsername   string
	onboardingURL string

	mu   sync.Mutex
	seen *boundedSet
}

func New(
	reviewer domain.Reviewer,
	poster domain.CommentPoster,
	logger *zap.Logger,
	botUsername string,
	onboardingURL string,
) *Processor {
	return &Processor{
		reviewer:      reviewer,
		poster:        poster,
		logger:        logger.With(zap.String("layer", "processor")),
		botUsername:   botUsername,
		onboardingURL: onboardingURL,
		seen:          newBoundedSet(seenCap),
	}
}

func (p *Processor) Process(ctx context.Context, data []byte) Result {
	var env event.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		p.logger.Warn("undecodable envelope; skipping", zap.Error(err))
		return Skip
	}

	var pr event.PullRequestEvent
	if err := json.Unmarshal(env.Payload, &pr); err != nil {
		p.logger.Warn("undecodable pull_request payload; skipping", zap.Error(err))
		return Skip
	}

	// Trigger only when a reviewer is added to a PR.
	if pr.Action != actionReviewerAdded {
		return Skip
	}

	// Only react when the bot itself is the requested reviewer. Compare
	// case-insensitively — GitFlame stores logins lowercased.
	if p.botUsername != "" && !strings.EqualFold(pr.RequestedReviewer.Login, p.botUsername) {
		p.logger.Debug(
			"requested_reviewer is not the bot; skipping",
			zap.String("requested_reviewer", pr.RequestedReviewer.Login),
			zap.String("bot_username", p.botUsername),
		)

		return Skip
	}

	assigner := pr.Sender.Login

	ref := domain.PRRef{
		Owner:    pr.Repository.Owner.Login,
		Repo:     pr.Repository.Name,
		Index:    pr.Index(),
		HeadSHA:  pr.PullRequest.Head.SHA,
		Assigner: assigner,
	}

	// Dedup by delivery_id (one Kafka re-delivery handled once).
	// Fall back to owner/repo/index@sha when delivery_id is absent (e.g. local tests).
	dedupKey := env.DeliveryID
	if dedupKey == "" {
		dedupKey = fmt.Sprintf("%s/%s/%d@%s", ref.Owner, ref.Repo, ref.Index, ref.HeadSHA)
	}

	p.mu.Lock()
	already := p.seen.has(dedupKey)
	p.mu.Unlock()

	if already {
		p.logger.Info("dedup hit; skipping", zap.String("key", dedupKey))
		return Skip
	}

	prLabel := fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Index)
	p.logger.Info("reviewing PR", zap.String("pr", prLabel), zap.String("assigner", assigner))

	review, err := p.reviewer.Review(ctx, ref)
	if err != nil {
		if errors.Is(err, domain.ErrNotOnboarded) {
			msg := fmt.Sprintf(
				"⚠️ @%s, connect your account at %s to use the review bot.",
				assigner, p.onboardingURL,
			)
			if postErr := p.poster.PostPRComment(ctx, ref, msg); postErr != nil {
				if errors.Is(postErr, domain.ErrTransient) {
					p.logger.Error("post not-onboarded comment transiently failed; will retry", zap.String("pr", prLabel), zap.Error(postErr))
					return Transient
				}

				p.logger.Error("post not-onboarded comment permanently failed; skipping", zap.String("pr", prLabel), zap.Error(postErr))
			}

			p.logger.Info("assigner not onboarded; skipped after comment", zap.String("pr", prLabel), zap.String("assigner", assigner))

			p.mu.Lock()
			p.seen.add(dedupKey, seenCap)
			p.mu.Unlock()

			return Skip
		}

		if errors.Is(err, domain.ErrPermanent) {
			// Permanent failure (e.g. 401/403 from LLM or GitFlame): commit the
			// offset so the partition advances rather than blocking indefinitely.
			p.logger.Error("review permanently failed; skipping message", zap.String("pr", prLabel), zap.Error(err))
			return Skip // intentional: Skip-after-permanent-error commits the offset
		}

		p.logger.Error("review failed; will retry", zap.Error(err))

		return Transient
	}

	if err := p.poster.PostPRComment(ctx, ref, review); err != nil {
		if errors.Is(err, domain.ErrPermanent) {
			// Same rationale: don't block the partition on a permanent poster error.
			p.logger.Error("post comment permanently failed; skipping message", zap.String("pr", prLabel), zap.Error(err))
			return Skip // intentional: Skip-after-permanent-error commits the offset
		}

		p.logger.Error("post comment failed; will retry", zap.Error(err))

		return Transient
	}

	p.mu.Lock()
	p.seen.add(dedupKey, seenCap)
	p.mu.Unlock()

	p.logger.Info("review posted", zap.String("pr", prLabel))

	return Done
}
