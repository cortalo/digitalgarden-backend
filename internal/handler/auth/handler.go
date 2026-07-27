// Package authhandler is the HTTP adapter for logging in: it only knows
// about JSON and gin, and depends on the auth service's use case, not on
// Google, Postgres, or JWT libraries directly.
package authhandler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	authservice "github.com/Cortalo/digitalgarden-backend/internal/service/auth"
)

type Service interface {
	LoginWithGoogle(ctx context.Context, rawGoogleToken string) (string, error)
}

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

type loginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// Login handles POST /api/auth/google.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := h.svc.LoginWithGoogle(c.Request.Context(), req.IDToken)
	if err != nil {
		if errors.Is(err, authservice.ErrUnauthorized) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid google token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, loginResponse{Token: token})
}
