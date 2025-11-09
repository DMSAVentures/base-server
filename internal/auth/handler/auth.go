package handler

import (
	"base-server/internal/apierrors"
	"base-server/internal/auth/processor"
	"base-server/internal/observability"
	"net/http"
	"net/url"
	"os"

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
		apierrors.RespondWithValidationError(c, err)
		return
	}

	token, err := h.authProcessor.Login(ctx, emailLoginRequest.Email, emailLoginRequest.Password)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
	return
}

func (h *Handler) HandleEmailSignup(c *gin.Context) {
	var req EmailSignupRequest
	ctx := c.Request.Context()

	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}
	signedUpUser, err := h.authProcessor.Signup(ctx, req.FirstName, req.LastName, req.Email, req.Password)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, signedUpUser)
	return
}

func (h *Handler) HandleJWTMiddleware(c *gin.Context) {
	ctx := c.Request.Context()
	token, err := c.Cookie("token")
	if token == "" || err != nil {
		apierrors.RespondWithError(c, apierrors.Unauthorized("Authorization token is missing or invalid"))
		c.Abort() // Stop further processing
		return
	}

	claims, err := h.authProcessor.ValidateJWTToken(ctx, token)

	if err != nil {
		apierrors.RespondWithError(c, apierrors.Unauthorized("Invalid or expired token"))
		c.Abort() // Stop further processing
		return
	}
	sub, err := claims.GetSubject()
	if err != nil {
		apierrors.RespondWithError(c, apierrors.Unauthorized("Invalid token claims"))
		c.Abort() // Stop further processing
		return
	}
	c.Set("User-ID", sub)
	c.Set("Account-ID", claims.AccountID)

	// Add auth context to observability for comprehensive logging
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "user_id", Value: sub},
		observability.Field{Key: "account_id", Value: claims.AccountID},
		observability.Field{Key: "auth_type", Value: claims.AuthType},
	)
	c.Request = c.Request.WithContext(ctx)

	// Continue to the next handler if the token is valid
	c.Next()
}

func (h *Handler) GetUserInfo(context *gin.Context) {
	ctx := context.Request.Context()
	userID, ok := context.Get("User-ID")
	if !ok {
		apierrors.RespondWithError(context, apierrors.Unauthorized("User ID not found in context"))
		return
	}

	parsedUserID, err := uuid.Parse(userID.(string))
	if err != nil {
		apierrors.RespondWithError(context, apierrors.BadRequest(apierrors.CodeInvalidInput, "Invalid user ID format"))
		return
	}
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: parsedUserID.String()})

	user, err := h.authProcessor.GetUserByExternalID(ctx, parsedUserID)
	if err != nil {
		apierrors.RespondWithError(context, err)
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
		apierrors.RespondWithError(c, apierrors.BadRequest(apierrors.CodeInvalidInput, "Authorization code is missing"))
		return
	}
	// Exchange the authorization code for access JWTToken
	JWTToken, err := h.authProcessor.SignInGoogleUserWithCode(ctx, code)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	if os.Getenv("GO_ENV") != "production" {
		c.SetCookie("token", JWTToken, 86400, "/", h.authProcessor.GetWebAppHost(), false, true)
	} else {
		c.SetCookie("token", JWTToken, 86400, "/", h.authProcessor.GetWebAppHost(), true, true)
	}

	parsedUrl, err := url.Parse(h.authProcessor.GetWebAppHost())
	if err != nil {
		apierrors.RespondWithError(c, apierrors.InternalError(err))
		return
	}

	redirectUrl := url.URL{
		Scheme: parsedUrl.Scheme,
		Host:   parsedUrl.Host,
		Path:   "/",
	}

	c.Redirect(http.StatusFound, redirectUrl.String())
	return
}
