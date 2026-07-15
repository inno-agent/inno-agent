package webhook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-webhook/internal/config"
	"github.com/inno-agent/inno-agent/backend/review-webhook/internal/webhook"
	"go.uber.org/zap"
)

// fakePublisher records the last call and can be set to return an error.
type fakePublisher struct {
	called bool
	key    string
	value  []byte
	err    error
}

func (f *fakePublisher) Publish(_ context.Context, key string, value []byte) error {
	f.called = true
	f.key = key
	f.value = value
	return f.err
}

func defaultCfg() *config.Config {
	return &config.Config{
		ServerPort:            "8002",
		KafkaBrokers:          "localhost:9092",
		KafkaTopic:            "gitflame.events",
		WebhookAuthorization:  "",
		WebhookAuthHeader:     "Authorization",
		WebhookEventHeader:    "X-GitFlame-Event",
		WebhookDeliveryHeader: "X-GitFlame-Delivery",
	}
}

func newHandler(cfg *config.Config, pub webhook.Publisher) http.Handler {
	return webhook.New(cfg, pub, zap.NewNop())
}

// TestAuthDisabled: when WebhookAuthorization is empty, any request is accepted.
func TestAuthDisabled(t *testing.T) {
	pub := &fakePublisher{}
	h := newHandler(defaultCfg(), pub)

	req := httptest.NewRequest(http.MethodPost, "/hooks/gitflame", bytes.NewBufferString(`{}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !pub.called {
		t.Fatal("expected publisher to be called")
	}
}

// TestAuthMissingHeader: when auth is required but header is absent → 401.
func TestAuthMissingHeader(t *testing.T) {
	pub := &fakePublisher{}
	cfg := defaultCfg()
	cfg.WebhookAuthorization = "secret-token"
	h := newHandler(cfg, pub)

	req := httptest.NewRequest(http.MethodPost, "/hooks/gitflame", bytes.NewBufferString(`{}`))
	// No Authorization header set.
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if pub.called {
		t.Fatal("publisher must NOT be called on auth failure")
	}
}

// TestAuthWrongHeader: wrong token value → 401.
func TestAuthWrongHeader(t *testing.T) {
	pub := &fakePublisher{}
	cfg := defaultCfg()
	cfg.WebhookAuthorization = "secret-token"
	h := newHandler(cfg, pub)

	req := httptest.NewRequest(http.MethodPost, "/hooks/gitflame", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "wrong-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	if pub.called {
		t.Fatal("publisher must NOT be called on auth failure")
	}
}

// TestAuthCorrect: correct token → 204; publisher receives Envelope with correct fields.
func TestAuthCorrect(t *testing.T) {
	pub := &fakePublisher{}
	cfg := defaultCfg()
	cfg.WebhookAuthorization = "secret-token"
	h := newHandler(cfg, pub)

	body := `{"repository":{"full_name":"owner/repo"},"action":"review_requested"}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/gitflame", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "secret-token")
	req.Header.Set("X-GitFlame-Event", "pull_request")
	req.Header.Set("X-GitFlame-Delivery", "abc-123")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !pub.called {
		t.Fatal("expected publisher to be called")
	}

	// Parse the envelope the publisher received.
	var env webhook.Envelope
	if err := json.Unmarshal(pub.value, &env); err != nil {
		t.Fatalf("could not unmarshal envelope: %v", err)
	}
	if env.EventType != "pull_request" {
		t.Errorf("event_type: want %q, got %q", "pull_request", env.EventType)
	}
	if env.DeliveryID != "abc-123" {
		t.Errorf("delivery_id: want %q, got %q", "abc-123", env.DeliveryID)
	}
	if string(env.Payload) != body {
		t.Errorf("payload mismatch: want %q, got %q", body, string(env.Payload))
	}
	if pub.key != "owner/repo" {
		t.Errorf("key: want %q, got %q", "owner/repo", pub.key)
	}
}

// TestPublishError: when publisher returns an error → 502.
func TestPublishError(t *testing.T) {
	pub := &fakePublisher{err: errors.New("broker unavailable")}
	h := newHandler(defaultCfg(), pub)

	req := httptest.NewRequest(http.MethodPost, "/hooks/gitflame", bytes.NewBufferString(`{}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rr.Code)
	}
}

// TestInvalidJSONBodyStillPublishes: non-JSON body → 204 with empty key (best-effort).
func TestInvalidJSONBodyStillPublishes(t *testing.T) {
	pub := &fakePublisher{}
	h := newHandler(defaultCfg(), pub)

	req := httptest.NewRequest(http.MethodPost, "/hooks/gitflame", bytes.NewBufferString(`not json at all`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if !pub.called {
		t.Fatal("expected publisher to be called even for non-JSON body")
	}
	if pub.key != "" {
		t.Errorf("key: want empty string, got %q", pub.key)
	}
}

// TestKeyFallbackOwnerLogin: no full_name but owner.login + name present → "owner/repo".
func TestKeyFallbackOwnerLogin(t *testing.T) {
	pub := &fakePublisher{}
	h := newHandler(defaultCfg(), pub)

	body := `{"repository":{"name":"repo","owner":{"login":"owner"}}}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/gitflame", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if pub.key != "owner/repo" {
		t.Errorf("key: want %q, got %q", "owner/repo", pub.key)
	}
}

func TestFormEncodedPayload(t *testing.T) {
	pub := &fakePublisher{}
	h := newHandler(defaultCfg(), pub)

	inner := `{"repository":{"full_name":"owner/repo"},"action":"assigned","number":3}`
	formBody := "payload=" + url.QueryEscape(inner)
	req := httptest.NewRequest(http.MethodPost, "/hooks/gitflame", bytes.NewBufferString(formBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-GitFlame-Event", "issues")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}

	var env webhook.Envelope
	if err := json.Unmarshal(pub.value, &env); err != nil {
		t.Fatalf("could not unmarshal envelope: %v", err)
	}
	if string(env.Payload) != inner {
		t.Errorf("payload mismatch: want %q, got %q", inner, string(env.Payload))
	}
	if pub.key != "owner/repo" {
		t.Errorf("key: want %q, got %q", "owner/repo", pub.key)
	}
}
