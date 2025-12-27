package apierrors

import (
	"errors"
	"strings"

	analyticsProcessor "base-server/internal/analytics/processor"
	authProcessor "base-server/internal/auth/processor"
	campaignProcessor "base-server/internal/campaign/processor"
	referralProcessor "base-server/internal/referral/processor"
	rewardsProcessor "base-server/internal/rewards/processor"
	"base-server/internal/store"
	waitlistProcessor "base-server/internal/waitlist/processor"
)

// MapError converts domain/processor errors to APIErrors.
// This function centralizes all error mapping logic to ensure consistent
// error responses across the entire API.
//
// If the error is already an APIError, it returns it as-is.
// If the error is a known domain error, it maps it to an appropriate APIError.
// If the error is unknown, it returns a sanitized InternalError (500).
func MapError(err error) *APIError {
	if err == nil {
		return nil
	}

	// Check if already an APIError
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}

	// Map auth processor errors
	switch {
	case errors.Is(err, authProcessor.ErrEmailAlreadyExists):
		return Conflict(CodeEmailExists, "Email already exists")

	case errors.Is(err, authProcessor.ErrEmailDoesNotExist):
		return NotFound(CodeEmailNotFound, "Email does not exist")

	case errors.Is(err, authProcessor.ErrIncorrectPassword):
		return Unauthorized("Invalid email or password")

	case errors.Is(err, authProcessor.ErrUserNotFound):
		return NotFound(CodeUserNotFound, "User not found")

	case errors.Is(err, authProcessor.ErrFailedSignup):
		return InternalError(err)

	case errors.Is(err, authProcessor.ErrFailedLogin):
		return InternalError(err)

	case errors.Is(err, authProcessor.ErrFailedGetUser):
		return InternalError(err)

	// Map campaign processor errors
	case errors.Is(err, campaignProcessor.ErrCampaignNotFound):
		return NotFound(CodeCampaignNotFound, "Campaign not found")

	case errors.Is(err, campaignProcessor.ErrSlugAlreadyExists):
		return Conflict(CodeSlugExists, "Campaign slug already exists")

	case errors.Is(err, campaignProcessor.ErrInvalidCampaignStatus):
		return BadRequest(CodeInvalidStatus, "Invalid campaign status")

	case errors.Is(err, campaignProcessor.ErrInvalidCampaignType):
		return BadRequest(CodeInvalidType, "Invalid campaign type")

	case errors.Is(err, campaignProcessor.ErrUnauthorized):
		return Forbidden("You do not have access to this campaign")

	// Map waitlist processor errors
	case errors.Is(err, waitlistProcessor.ErrCampaignNotFound):
		return NotFound(CodeCampaignNotFound, "Campaign not found")

	case errors.Is(err, waitlistProcessor.ErrEmailAlreadyExists):
		return Conflict(CodeEmailExists, "Email already exists for this campaign")

	case errors.Is(err, waitlistProcessor.ErrInvalidReferralCode):
		return BadRequest(CodeInvalidReferral, "Invalid referral code")

	case errors.Is(err, waitlistProcessor.ErrInvalidStatus):
		return BadRequest(CodeInvalidStatus, "Invalid user status")

	case errors.Is(err, waitlistProcessor.ErrInvalidSource):
		return BadRequest(CodeInvalidSource, "Invalid user source. Valid values: direct, referral, social, ad")

	case errors.Is(err, waitlistProcessor.ErrUnauthorized):
		return Forbidden("You do not have access to this campaign")

	case errors.Is(err, waitlistProcessor.ErrInvalidVerificationToken):
		return BadRequest(CodeInvalidToken, "Invalid or expired verification token")

	case errors.Is(err, waitlistProcessor.ErrEmailAlreadyVerified):
		return Conflict(CodeAlreadyVerified, "Email already verified")

	case errors.Is(err, waitlistProcessor.ErrMaxSignupsReached):
		return BadRequest(CodeMaxSignupsReached, "Campaign has reached maximum signups")

	case errors.Is(err, waitlistProcessor.ErrUserNotFound):
		return NotFound(CodeUserNotFound, "User not found")

	case errors.Is(err, waitlistProcessor.ErrCaptchaRequired):
		return BadRequest(CodeCaptchaRequired, "Captcha verification required")

	case errors.Is(err, waitlistProcessor.ErrCaptchaFailed):
		return BadRequest(CodeCaptchaFailed, "Captcha verification failed")

	case errors.Is(err, waitlistProcessor.ErrCampaignNotActive):
		return BadRequest(CodeCampaignNotActive, "This campaign is not currently accepting signups")

	// Map rewards processor errors
	case errors.Is(err, rewardsProcessor.ErrRewardNotFound):
		return NotFound(CodeRewardNotFound, "Reward not found")

	case errors.Is(err, rewardsProcessor.ErrUserRewardNotFound):
		return NotFound(CodeUserRewardNotFound, "User reward not found")

	case errors.Is(err, rewardsProcessor.ErrInvalidRewardType):
		return BadRequest(CodeInvalidRewardType, "Invalid reward type")

	case errors.Is(err, rewardsProcessor.ErrInvalidTriggerType):
		return BadRequest(CodeInvalidTriggerType, "Invalid trigger type")

	case errors.Is(err, rewardsProcessor.ErrInvalidDeliveryMethod):
		return BadRequest(CodeInvalidDeliveryMethod, "Invalid delivery method")

	case errors.Is(err, rewardsProcessor.ErrInvalidRewardStatus):
		return BadRequest(CodeInvalidRewardStatus, "Invalid reward status")

	case errors.Is(err, rewardsProcessor.ErrUnauthorized):
		return Forbidden("You do not have access to this reward")

	case errors.Is(err, rewardsProcessor.ErrRewardLimitReached):
		return BadRequest(CodeRewardLimitReached, "Reward limit has been reached")

	case errors.Is(err, rewardsProcessor.ErrUserLimitReached):
		return BadRequest(CodeUserLimitReached, "You have already claimed the maximum number of rewards")

	// Map referral processor errors
	case errors.Is(err, referralProcessor.ErrReferralNotFound):
		return NotFound(CodeReferralNotFound, "Referral not found")

	case errors.Is(err, referralProcessor.ErrCampaignNotFound):
		return NotFound(CodeCampaignNotFound, "Campaign not found")

	case errors.Is(err, referralProcessor.ErrUserNotFound):
		return NotFound(CodeUserNotFound, "User not found")

	case errors.Is(err, referralProcessor.ErrUnauthorized):
		return Forbidden("You do not have access to this campaign")

	case errors.Is(err, referralProcessor.ErrInvalidStatus):
		return BadRequest(CodeInvalidStatus, "Invalid referral status")

	case errors.Is(err, referralProcessor.ErrInvalidReferral):
		return BadRequest(CodeInvalidReferral, "Invalid referral code")

	case errors.Is(err, referralProcessor.ErrReferralCodeEmpty):
		return BadRequest(CodeReferralCodeRequired, "Referral code is required")

	// Map analytics processor errors
	case errors.Is(err, analyticsProcessor.ErrCampaignNotFound):
		return NotFound(CodeCampaignNotFound, "Campaign not found")

	case errors.Is(err, analyticsProcessor.ErrUnauthorized):
		return Forbidden("You do not have access to this campaign")

	case errors.Is(err, analyticsProcessor.ErrInvalidDateRange):
		return BadRequest(CodeInvalidDateRange, "Invalid date range")

	case errors.Is(err, analyticsProcessor.ErrInvalidGranularity):
		return BadRequest(CodeInvalidGranularity, "Invalid granularity")

	// Map store errors
	case errors.Is(err, store.ErrNotFound):
		return NotFound(CodeNotFound, "Resource not found")

	// Check for common external service errors by message content
	default:
		return mapExternalServiceError(err)
	}
}

// mapExternalServiceError attempts to identify external service errors
// and map them to appropriate service-specific error responses.
func mapExternalServiceError(err error) *APIError {
	errMsg := strings.ToLower(err.Error())

	// Stripe/payment errors
	if strings.Contains(errMsg, "stripe") || strings.Contains(errMsg, "payment") {
		return ServiceUnavailable(
			CodePaymentProviderError,
			"Payment provider is temporarily unavailable. Please try again later.",
			err,
		)
	}

	// Email service errors (Resend)
	if strings.Contains(errMsg, "resend") || strings.Contains(errMsg, "email service") {
		return ServiceUnavailable(
			CodeEmailServiceError,
			"Email service is temporarily unavailable. Please try again later.",
			err,
		)
	}

	// AI service errors (OpenAI, Gemini)
	if strings.Contains(errMsg, "openai") || strings.Contains(errMsg, "gemini") || strings.Contains(errMsg, "ai service") {
		return ServiceUnavailable(
			CodeAIServiceError,
			"AI service is temporarily unavailable. Please try again later.",
			err,
		)
	}

	// Default: Unknown error - return sanitized 500
	return InternalError(err)
}
