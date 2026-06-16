package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
)

type reviewRequest struct {
	PRID string `json:"pr_id"`
	Diff string `json:"diff,omitempty"`
}

type reviewResponse struct {
	ReviewMarkdown string `json:"review_markdown"`
}

// ReviewHandler handles HTTP requests for AI pull request reviews.
type ReviewHandler struct {
	service domain.ReviewService
	logger  *zap.Logger
}

// NewReviewHandler creates a ReviewHandler with the given service and logger.
func NewReviewHandler(service domain.ReviewService, logger *zap.Logger) *ReviewHandler {
	return &ReviewHandler{service: service, logger: logger}
}

// Review accepts a POST request with a PR identifier and returns a markdown review.
func (h *ReviewHandler) Review(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("invalid request body", zap.Error(err))
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.PRID == "" {
		writeError(w, http.StatusBadRequest, "pr_id is required")
		return
	}

	review, err := h.service.ReviewPR(ctx, req.PRID, req.Diff)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			writeError(w, http.StatusBadRequest, "pr_id is required")
		case errors.Is(err, domain.ErrDiffUnavailable):
			writeError(w, http.StatusBadRequest, "diff is required when GitFlame is unavailable")
		default:
			h.logger.Error("failed to review PR", zap.String("pr_id", req.PRID), zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to generate review")
		}
		return
	}

	writeJSON(w, http.StatusOK, reviewResponse{ReviewMarkdown: review})
}
