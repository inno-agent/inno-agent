package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	reviewer domain.Reviewer
	poster   domain.CommentPoster
	logger   *zap.Logger

	mu   sync.Mutex
	seen *boundedSet
}

func New(reviewer domain.Reviewer, poster domain.CommentPoster, logger *zap.Logger) *Processor {
	return &Processor{
		reviewer: reviewer,
		poster:   poster,
		logger:   logger.With(zap.String("layer", "processor")),
		seen:     newBoundedSet(seenCap),
	}
}

func (p *Processor) Process(ctx context.Context, data []byte) Result {
	var env event.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		p.logger.Warn("undecodable envelope; skipping", zap.Error(err))
		return Skip
	}

	if env.EventType != "pull_request" {
		return Skip
	}

	var pr event.PullRequestEvent
	if err := json.Unmarshal(env.Payload, &pr); err != nil {
		p.logger.Warn("undecodable pull_request payload; skipping", zap.Error(err))
		return Skip
	}

	if pr.Action != "opened" && pr.Action != "synchronized" {
		return Skip
	}

	ref := domain.PRRef{
		Owner:   pr.Repository.Owner.Login,
		Repo:    pr.Repository.Name,
		Index:   pr.Index(), // Fix 4: prefer nested pull_request.number
		HeadSHA: pr.PullRequest.Head.SHA,
	}

	dedupKey := fmt.Sprintf("%s/%s/%d@%s", ref.Owner, ref.Repo, ref.Index, ref.HeadSHA)
	p.mu.Lock()
	already := p.seen.has(dedupKey)
	p.mu.Unlock()
	if already {
		p.logger.Info("dedup hit; skipping", zap.String("key", dedupKey))
		return Skip
	}

	prLabel := fmt.Sprintf("%s/%s#%d", ref.Owner, ref.Repo, ref.Index)
	p.logger.Info("reviewing PR", zap.String("pr", prLabel))

	review, err := p.reviewer.Review(ctx, ref)
	if err != nil {
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
