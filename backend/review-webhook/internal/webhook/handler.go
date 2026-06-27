package webhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-webhook/internal/config"
)

const maxBodyBytes = 8 * 1024 * 1024 // 8 MiB

// Publisher publishes a single Kafka message.
type Publisher interface {
	Publish(ctx context.Context, key string, value []byte) error
}

// Envelope is the wire contract shared with the consumer.
type Envelope struct {
	DeliveryID string          `json:"delivery_id"`
	EventType  string          `json:"event_type"`
	Payload    json.RawMessage `json:"payload"`
}

// repoKeyPayload is used for best-effort key extraction only.
type repoKeyPayload struct {
	Repository struct {
		FullName string `json:"full_name"`
		Name     string `json:"name"`
		Owner    struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

// Handler handles incoming GitFlame webhook deliveries.
type Handler struct {
	cfg       *config.Config
	publisher Publisher
	logger    *zap.Logger
}

// New creates a new Handler.
func New(cfg *config.Config, pub Publisher, logger *zap.Logger) *Handler {
	return &Handler{cfg: cfg, publisher: pub, logger: logger}
}

// ServeHTTP implements http.Handler for POST /hooks/gitflame.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_error")
		return
	}

	// Auth check.
	if h.cfg.WebhookAuthorization != "" {
		got := r.Header.Get(h.cfg.WebhookAuthHeader)
		want := h.cfg.WebhookAuthorization
		if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
	}

	key := extractKey(body)

	// Use the raw body as-is if it is valid JSON; otherwise encode it as a
	// JSON string so the Envelope itself is always valid JSON.
	payload := json.RawMessage(body)
	if !json.Valid(body) {
		encoded, _ := json.Marshal(string(body))
		payload = json.RawMessage(encoded)
	}

	envelope := Envelope{
		DeliveryID: r.Header.Get(h.cfg.WebhookDeliveryHeader),
		EventType:  r.Header.Get(h.cfg.WebhookEventHeader),
		Payload:    payload,
	}

	value, err := json.Marshal(envelope)
	if err != nil {
		h.logger.Error("marshal envelope failed", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	if err := h.publisher.Publish(r.Context(), key, value); err != nil {
		h.logger.Error("publish failed", zap.Error(err), zap.String("key", key))
		writeError(w, http.StatusBadGateway, "publish_failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// extractKey does a best-effort parse to get "owner/repo" from the payload.
// On any failure it returns an empty string — events are never dropped over a key.
func extractKey(body []byte) string {
	var p repoKeyPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return ""
	}
	if p.Repository.FullName != "" {
		return p.Repository.FullName
	}
	if p.Repository.Owner.Login != "" && p.Repository.Name != "" {
		return p.Repository.Owner.Login + "/" + p.Repository.Name
	}
	return ""
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
