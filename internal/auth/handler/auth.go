package handler

import (
	"base-server/internal/auth/processor"
	"base-server/internal/observability"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	token, err := h.authProcessor.Login(ctx, emailLoginRequest.Email, emailLoginRequest.Password)
	if err != nil {
		h.logger.Error(ctx, "failed to login", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
	return
}

func (h *Handler) HandleEmailSignup(c *gin.Context) {
	var req EmailSignupRequest
	ctx := c.Request.Context()

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(ctx, "failed to bind request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
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

func (h *Handler) HandleJWTMiddleware(c *gin.Context) {
	ctx := c.Request.Context()
	tokeHeader := c.GetHeader("Authorization")

	if tokeHeader == "" || !strings.HasPrefix(tokeHeader, "Bearer ") {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token is missing or invalid"})
		c.Abort() // Stop further processing
		return
	}

	// Extract the JWT token from the header
	tokenString := strings.TrimPrefix(tokeHeader, "Bearer ")

	claims, err := h.authProcessor.ValidateJWTToken(ctx, tokenString)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		c.Abort() // Stop further processing
		return
	}
	sub, err := claims.GetSubject()
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		c.Abort() // Stop further processing
		return
	}
	c.Set("User-ID", sub)
	// Continue to the next handler if the token is valid
	c.Next()
}

func (h *Handler) GetUserInfo(context *gin.Context) {
	ctx := context.Request.Context()
	userID, ok := context.Get("User-ID")
	if !ok {
		h.logger.Error(ctx, "failed to get userID from context", nil)
		context.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
		return
	}

	parsedUserID, err := uuid.Parse(userID.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse userID id", err)
		context.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id"})
		return
	}
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: parsedUserID.String()})

	user, err := h.authProcessor.GetUserByExternalID(ctx, parsedUserID)
	if err != nil {
		h.logger.Error(ctx, "failed to get userID by external id", err)
		context.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	context.JSON(http.StatusOK, user)
	return
}

func (h *Handler) HandleGoogleOauthCallback(c *gin.Context) {
	ctx := c.Request.Context()
	// Extract the authorization code from the query parameters
	code := c.Request.URL.Query().Get("code")
	if code == "" {
		h.logger.Error(ctx, "authorization code is missing", nil)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization code is missing"})
		return
	}
	// Exchange the authorization code for access JWTToken
	JWTToken, err := h.authProcessor.SignInGoogleUserWithCode(ctx, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	redirectUrl := url.URL{
		Host: h.authProcessor.GetWebAppHost(),
		Path: "oauth/signedin",
	}
	query := redirectUrl.Query()
	query.Add("token", JWTToken)
	redirectUrl.RawQuery = query.Encode()

	c.Redirect(http.StatusFound, redirectUrl.String())
	return
}
