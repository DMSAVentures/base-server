package handler

import (
	"base-server/internal/auth/processor"
	"base-server/internal/observability"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	authProcessor processor.AuthProcessor
	logger        *observability.Logger
}

type EmailSignupRequest struct {
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
}

type EmailLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func New(authProcessor processor.AuthProcessor, logger *observability.Logger) Handler {
	return Handler{authProcessor: authProcessor, logger: logger}
}

func (h *Handler) HandleEmailLogin(c *gin.Context) {
	var emailLoginRequest EmailLoginRequest
	ctx := c.Request.Context()
	if err := c.ShouldBindJSON(&emailLoginRequest); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	loggedInUser, err := h.authProcessor.Login(ctx, emailLoginRequest.Email, emailLoginRequest.Password)
	if err != nil {
		h.logger.Error(ctx, "failed to login", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return

	}
	c.JSON(http.StatusOK, loggedInUser)
	return
}

func (h *Handler) HandleEmailSignup(c *gin.Context) {
	var req EmailSignupRequest
	ctx := c.Request.Context()
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	signedUpUser, err := h.authProcessor.Signup(ctx, req.FirstName, req.LastName, req.Email, req.Password)
	if err != nil {
		h.logger.Error(ctx, "failed to signup", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, signedUpUser)
	return
}

func (h *Handler) HandleOAuthSignup(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"message": "FOUND IT",
	})
	return
}

func (h *Handler) HandleOAuthLogin(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"message": "FOUND IT",
	})
	return
}
