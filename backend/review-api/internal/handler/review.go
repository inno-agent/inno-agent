package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/middleware"
)

type reviewRequest struct {
	PRID  string `json:"pr_id"`
	Diff  string `json:"diff,omitempty"`
	Model string `json:"model,omitempty"`
}

type reviewResponse struct {
	ReviewMarkdown string `json:"review_markdown"`
}

// ReviewHandler handles HTTP requests for AI pull request reviews.
type ReviewHandler struct {
	service domain.ReviewService
}

// NewReviewHandler creates a ReviewHandler with the given service.
func NewReviewHandler(service domain.ReviewService) *ReviewHandler {
	return &ReviewHandler{service: service}
}

// Review accepts a POST request with a PR identifier and returns a markdown review.
func (h *ReviewHandler) Review(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.LoggerFromContext(ctx).Error("invalid request body", zap.Error(err))
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.PRID == "" {
		writeError(w, http.StatusBadRequest, "pr_id is required")
		return
	}

	review, err := h.service.ReviewPR(ctx, req.PRID, req.Diff, req.Model)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrValidation):
			writeError(w, http.StatusBadRequest, "invalid pr_id format")
		case errors.Is(err, domain.ErrDiffUnavailable):
			writeError(w, http.StatusBadGateway, "failed to fetch PR diff from upstream")
		default:
			middleware.LoggerFromContext(ctx).Error("failed to review PR", zap.String("pr_id", req.PRID), zap.Error(err))
			writeError(w, http.StatusInternalServerError, "failed to generate review")
		}
		return
	}

	writeJSON(w, http.StatusOK, reviewResponse{ReviewMarkdown: review})
}
