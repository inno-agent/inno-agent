package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"innoagent/internal/auth"
	"innoagent/internal/orchestrator"

	"go.uber.org/zap"
)

// completionsService is the slice of the orchestrator the handler needs.
type completionsService interface {
	Complete(ctx context.Context, body []byte) (*orchestrator.CompleteResult, error)
}

// completionsHandler serves POST /v1/chat/completions. Authentication is done
// by auth.Middleware, which wraps this handler and puts user_id in the context.
func completionsHandler(svc completionsService, maxBodyBytes int64, log *zap.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		start := time.Now()

		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
				return
			}
			writeJSONError(w, http.StatusBadRequest, "could not read request body")
			return
		}

		res, err := svc.Complete(r.Context(), body)
		if err != nil {
			status := http.StatusInternalServerError
			msg := "internal error"
			switch {
			case errors.Is(err, orchestrator.ErrInvalidBody):
				status, msg = http.StatusBadRequest, "invalid request body"
			case errors.Is(err, orchestrator.ErrEmptyMessages):
				status, msg = http.StatusBadRequest, "messages field is required"
			case errors.Is(err, orchestrator.ErrStreamUnsupported):
				status, msg = http.StatusNotImplemented, "streaming is not supported on this endpoint"
			case errors.Is(err, orchestrator.ErrModelNotAllowed):
				status, msg = http.StatusForbidden, "model not allowed"
			default:
				// Deliberately generic: an unexpected error may carry internal
				// addresses or paths. The detail goes to the log only.
				log.Error("completions handler error", zap.Error(err))
			}
			writeJSONError(w, status, msg)
			return
		}

		// Per-user attribution: this line is the accounting the direct-to-Ollama
		// path never had. Until the delegated-token work lands, user_id is a
		// service identity rather than a person.
		log.Info("llm_completion",
			zap.String("user_id", auth.UserIDFromContext(r.Context())),
			zap.String("requested_model", res.RequestedModel),
			zap.String("resolved_model", res.ResolvedModel),
			zap.Int("request_bytes", len(body)),
			zap.Int("status", res.Status),
			zap.Duration("duration", time.Since(start)))

		w.Header().Set("Content-Type", res.ContentType)
		w.WriteHeader(res.Status)
		if _, err := w.Write(res.Body); err != nil {
			log.Error("failed to write completions response", zap.Error(err))
		}
	})
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"message": message},
	})
}
