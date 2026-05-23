package api

import (
	"encoding/json"
	"net/http"

	"source-asia-assignment/internal/ratelimit"

	"github.com/gin-gonic/gin"
)

type RequestHandler struct {
	limiter *ratelimit.Limiter
}

func NewRequestHandler(l *ratelimit.Limiter) *RequestHandler {
	return &RequestHandler{limiter: l}
}

type incomingRequest struct {
	UserID  string          `json:"user_id"`
	Payload json.RawMessage `json:"payload"`
}

// HandleRequest is POST /request — the rate-limited endpoint.
func (h *RequestHandler) HandleRequest(c *gin.Context) {
	var req incomingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.UserID == "" {
		jsonError(c, http.StatusBadRequest, "user_id is required and must be non-empty")
		return
	}

	// json.RawMessage is nil when the key is absent; a JSON null becomes
	// the four bytes []byte("null"), which is fine — null is a valid value.
	if len(req.Payload) == 0 {
		jsonError(c, http.StatusBadRequest, "payload is required")
		return
	}

	if !h.limiter.Allow(req.UserID) {
		jsonError(c, http.StatusTooManyRequests,
			"rate limit exceeded: max 5 requests per minute per user")
		return
	}

	jsonOK(c, http.StatusCreated, gin.H{
		"message": "request accepted",
		"user_id": req.UserID,
	})
}

// HandleStats is GET /stats — returns per-user and global counters.
func (h *RequestHandler) HandleStats(c *gin.Context) {
	stats := h.limiter.Stats()
	jsonOK(c, http.StatusOK, stats)
}
