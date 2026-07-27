package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

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
		if !authorizationMatches(got, want) {
			h.logger.Warn(
				"webhook rejected: authorization "+authorizationStatus(got, want),
				zap.String("delivery_id", firstHeader(
					r,
					h.cfg.WebhookDeliveryHeader,
					"X-GitFlame-Delivery",
					"X-Gitea-Delivery",
					"X-GitHub-Delivery",
				)),
				zap.String("event_type", firstHeader(
					r,
					h.cfg.WebhookEventHeader,
					"X-GitFlame-Event",
					"X-Gitea-Event",
					"X-GitHub-Event",
				)),
			)
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
	}

	key := extractKey(body)

	payload := extractPayload(body, r.Header.Get("Content-Type"))

	envelope := Envelope{
		DeliveryID: firstHeader(
			r,
			h.cfg.WebhookDeliveryHeader,
			"X-GitFlame-Delivery",
			"X-Gitea-Delivery",
			"X-GitHub-Delivery",
		),
		EventType: firstHeader(
			r,
			h.cfg.WebhookEventHeader,
			"X-GitFlame-Event",
			"X-GitFlame-Event-Type",
			"X-Gitea-Event",
			"X-Gitea-Event-Type",
			"X-GitHub-Event",
			"X-GitHub-Event-Type",
		),
		Payload: payload,
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

	h.logger.Info(
		"webhook published",
		zap.String("event_type", envelope.EventType),
		zap.String("delivery_id", envelope.DeliveryID),
		zap.String("key", key),
	)

	w.WriteHeader(http.StatusNoContent)
}

func extractPayload(body []byte, contentType string) json.RawMessage {
	body = bytesTrimSpace(body)
	if len(body) == 0 {
		return json.RawMessage("{}")
	}

	if json.Valid(body) && (body[0] == '{' || body[0] == '[') {
		return json.RawMessage(body)
	}

	ct := strings.ToLower(contentType)
	if strings.Contains(ct, "application/x-www-form-urlencoded") || strings.Contains(string(body), "payload=") {
		if values, err := url.ParseQuery(string(body)); err == nil {
			if p := strings.TrimSpace(values.Get("payload")); p != "" {
				if json.Valid([]byte(p)) {
					return json.RawMessage(p)
				}
			}
		}
	}

	encoded, _ := json.Marshal(string(body))
	return json.RawMessage(encoded)
}

func firstHeader(r *http.Request, names ...string) string {
	for _, name := range names {
		if v := strings.TrimSpace(r.Header.Get(name)); v != "" {
			return v
		}
	}
	return ""
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// extractKey does a best-effort parse to get "owner/repo" from the payload.
// On any failure it returns an empty string — events are never dropped over a key.
func extractKey(body []byte) string {
	payload := extractPayload(body, "")
	var p repoKeyPayload
	if err := json.Unmarshal(payload, &p); err != nil {
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
