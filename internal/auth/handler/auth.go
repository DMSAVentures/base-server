package handler

import (
	"errors"
	"net/http"
	"net/url"
	"os"

	"base-server/internal/apierrors"
	"base-server/internal/auth/processor"
	"base-server/internal/observability"
	"base-server/internal/tiers"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	authProcessor processor.AuthProcessor
	tierService   *tiers.TierService
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

func New(authProcessor processor.AuthProcessor, tierService *tiers.TierService, logger *observability.Logger) Handler {
	return Handler{authProcessor: authProcessor, tierService: tierService, logger: logger}
}

// UserInfoResponse combines user info with tier information
type UserInfoResponse struct {
	FirstName  string          `json:"first_name"`
	LastName   string          `json:"last_name"`
	ExternalID uuid.UUID       `json:"external_id"`
	Tier       *tiers.TierInfo `json:"tier,omitempty"`
}

func (h *Handler) HandleEmailLogin(c *gin.Context) {
	var emailLoginRequest EmailLoginRequest
	ctx := c.Request.Context()
	if err := c.ShouldBindJSON(&emailLoginRequest); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	token, err := h.authProcessor.Login(ctx, emailLoginRequest.Email, emailLoginRequest.Password)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *Handler) HandleEmailSignup(c *gin.Context) {
	var req EmailSignupRequest
	ctx := c.Request.Context()

	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}
	signedUpUser, err := h.authProcessor.Signup(ctx, req.FirstName, req.LastName, req.Email, req.Password)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, signedUpUser)
}

func (h *Handler) HandleJWTMiddleware(c *gin.Context) {
	ctx := c.Request.Context()
	token, err := c.Cookie("token")
	if token == "" || err != nil {
		apierrors.Unauthorized(c, "Authorization token is missing or invalid")
		c.Abort()
		return
	}

	claims, err := h.authProcessor.ValidateJWTToken(ctx, token)

	if err != nil {
		apierrors.Unauthorized(c, "Invalid or expired token")
		c.Abort()
		return
	}
	sub, err := claims.GetSubject()
	if err != nil {
		apierrors.Unauthorized(c, "Invalid token claims")
		c.Abort()
		return
	}
	c.Set("User-ID", sub)
	c.Set("Account-ID", claims.AccountID)

	ctx = observability.WithFields(ctx,
		observability.Field{Key: "user_id", Value: sub},
		observability.Field{Key: "account_id", Value: claims.AccountID},
		observability.Field{Key: "auth_type", Value: claims.AuthType},
	)
	c.Request = c.Request.WithContext(ctx)

	c.Next()
}

func (h *Handler) GetUserInfo(c *gin.Context) {
	ctx := c.Request.Context()
	userID, ok := c.Get("User-ID")
	if !ok {
		apierrors.Unauthorized(c, "User ID not found in context")
		return
	}

	parsedUserID, err := uuid.Parse(userID.(string))
	if err != nil {
		apierrors.BadRequest(c, "INVALID_INPUT", "Invalid user ID format")
		return
	}
	ctx = observability.WithFields(ctx, observability.Field{Key: "user_id", Value: parsedUserID.String()})

	user, err := h.authProcessor.GetUserByExternalID(ctx, parsedUserID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	response := UserInfoResponse{
		FirstName:  user.FirstName,
		LastName:   user.LastName,
		ExternalID: user.ExternalID,
	}

	if h.tierService != nil {
		accountIDStr, accountOk := c.Get("Account-ID")
		if accountOk && accountIDStr != "" {
			accountID, parseErr := uuid.Parse(accountIDStr.(string))
			if parseErr == nil {
				tierInfo, tierErr := h.tierService.GetTierInfoByAccountID(ctx, accountID)
				if tierErr == nil {
					response.Tier = &tierInfo
				} else {
					h.logger.Error(ctx, "failed to get tier info", tierErr)
				}
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) HandleGoogleOauthCallback(c *gin.Context) {
	ctx := c.Request.Context()
	code := c.Request.URL.Query().Get("code")
	if code == "" {
		apierrors.BadRequest(c, "INVALID_INPUT", "Authorization code is missing")
		return
	}
	JWTToken, err := h.authProcessor.SignInGoogleUserWithCode(ctx, code)
	if err != nil {
		h.handleError(c, err)
		return
	}

	if os.Getenv("GO_ENV") != "production" {
		c.SetCookie("token", JWTToken, 86400, "/", h.authProcessor.GetWebAppHost(), false, true)
	} else {
		c.SetCookie("token", JWTToken, 86400, "/", h.authProcessor.GetWebAppHost(), true, true)
	}

	parsedUrl, err := url.Parse(h.authProcessor.GetWebAppHost())
	if err != nil {
		apierrors.InternalError(c, err)
		return
	}

	redirectUrl := url.URL{
		Scheme: parsedUrl.Scheme,
		Host:   parsedUrl.Host,
		Path:   "/",
	}

	c.Redirect(http.StatusFound, redirectUrl.String())
}

func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, processor.ErrEmailAlreadyExists):
		apierrors.Conflict(c, "EMAIL_EXISTS", "Email already exists")
	case errors.Is(err, processor.ErrEmailDoesNotExist):
		apierrors.NotFound(c, "Email does not exist")
	case errors.Is(err, processor.ErrIncorrectPassword):
		apierrors.Unauthorized(c, "Invalid email or password")
	case errors.Is(err, processor.ErrUserNotFound):
		apierrors.NotFound(c, "User not found")
	default:
		apierrors.InternalError(c, err)
	}
}
