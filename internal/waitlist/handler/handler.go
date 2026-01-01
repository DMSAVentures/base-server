package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"base-server/internal/apierrors"
	"base-server/internal/observability"
	"base-server/internal/waitlist/processor"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	processor          processor.WaitlistProcessor
	positionCalculator *processor.PositionCalculator
	logger             *observability.Logger
	baseURL            string
}

func New(
	processor processor.WaitlistProcessor,
	positionCalculator *processor.PositionCalculator,
	logger *observability.Logger,
	baseURL string,
) Handler {
	return Handler{
		processor:          processor,
		positionCalculator: positionCalculator,
		logger:             logger,
		baseURL:            baseURL,
	}
}

// handleError maps processor errors to appropriate HTTP responses
func (h *Handler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, processor.ErrUserNotFound):
		apierrors.NotFound(c, "User not found")
	case errors.Is(err, processor.ErrCampaignNotFound):
		apierrors.NotFound(c, "Campaign not found")
	case errors.Is(err, processor.ErrEmailAlreadyExists):
		apierrors.Conflict(c, "EMAIL_EXISTS", "Email already exists for this campaign")
	case errors.Is(err, processor.ErrInvalidReferralCode):
		apierrors.BadRequest(c, "INVALID_REFERRAL_CODE", "Invalid referral code")
	case errors.Is(err, processor.ErrInvalidStatus):
		apierrors.BadRequest(c, "INVALID_STATUS", "Invalid user status")
	case errors.Is(err, processor.ErrInvalidSource):
		apierrors.BadRequest(c, "INVALID_SOURCE", "Invalid user source")
	case errors.Is(err, processor.ErrUnauthorized):
		apierrors.Forbidden(c, "UNAUTHORIZED", "Unauthorized access to campaign")
	case errors.Is(err, processor.ErrInvalidVerificationToken):
		apierrors.BadRequest(c, "INVALID_TOKEN", "Invalid verification token")
	case errors.Is(err, processor.ErrEmailAlreadyVerified):
		apierrors.Conflict(c, "ALREADY_VERIFIED", "Email already verified")
	case errors.Is(err, processor.ErrMaxSignupsReached):
		apierrors.Conflict(c, "MAX_SIGNUPS_REACHED", "Campaign has reached maximum signups")
	case errors.Is(err, processor.ErrCaptchaRequired):
		apierrors.BadRequest(c, "CAPTCHA_REQUIRED", "Captcha verification required")
	case errors.Is(err, processor.ErrCaptchaFailed):
		apierrors.BadRequest(c, "CAPTCHA_FAILED", "Captcha verification failed")
	case errors.Is(err, processor.ErrCampaignNotActive):
		apierrors.Conflict(c, "CAMPAIGN_NOT_ACTIVE", "Campaign is not accepting signups")
	case errors.Is(err, processor.ErrJSONExportNotAvailable):
		apierrors.Forbidden(c, "FEATURE_NOT_AVAILABLE", "JSON export is not available in your plan")
	case errors.Is(err, processor.ErrLeadsLimitReached):
		apierrors.Forbidden(c, "LEADS_LIMIT_REACHED", "You have reached your leads limit. Please upgrade your plan to add more leads.")
	case errors.Is(err, processor.ErrEmailVerificationNotAvailable):
		apierrors.Forbidden(c, "FEATURE_NOT_AVAILABLE", "Email verification is not available in your plan. Please upgrade to Pro or Team plan.")
	case errors.Is(err, processor.ErrCustomFieldFilteringUnavailable):
		apierrors.Forbidden(c, "FEATURE_NOT_AVAILABLE", "Custom field filtering requires enhanced lead data feature. Please upgrade to Pro or Team plan.")
	default:
		apierrors.InternalError(c, err)
	}
}

// SignupRequest represents the HTTP request for signing up a user
type SignupRequest struct {
	Email            string            `json:"email" binding:"required,email"`
	ReferralCode     *string           `json:"referral_code,omitempty"`
	CustomFields     map[string]string `json:"custom_fields,omitempty"`
	UTMSource        *string           `json:"utm_source,omitempty"`
	UTMMedium        *string           `json:"utm_medium,omitempty"`
	UTMCampaign      *string           `json:"utm_campaign,omitempty"`
	UTMTerm          *string           `json:"utm_term,omitempty"`
	UTMContent       *string           `json:"utm_content,omitempty"`
	MarketingConsent bool              `json:"marketing_consent"`
	TermsAccepted    bool              `json:"terms_accepted" binding:"required"`
	CaptchaToken     *string           `json:"captcha_token,omitempty"`
}

// HandleSignupUser handles POST /api/v1/campaigns/:campaign_id/users (public endpoint)
func (h *Handler) HandleSignupUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	// Get real IP and User-Agent (from CloudFront headers if available)
	ipAddress := observability.GetRealClientIP(c)
	userAgent := observability.GetRealUserAgent(c)

	// Extract CloudFront viewer info (already returns *string and *float64)
	cfInfo := observability.GetCloudFrontViewerInfo(c)

	processorReq := processor.SignupUserRequest{
		Email:            req.Email,
		FirstName:        nil,
		LastName:         nil,
		ReferralCode:     req.ReferralCode,
		CustomFields:     req.CustomFields,
		UTMSource:        req.UTMSource,
		UTMMedium:        req.UTMMedium,
		UTMCampaign:      req.UTMCampaign,
		UTMTerm:          req.UTMTerm,
		UTMContent:       req.UTMContent,
		MarketingConsent: req.MarketingConsent,
		TermsAccepted:    req.TermsAccepted,
		IPAddress:        &ipAddress,
		UserAgent:        &userAgent,
		CaptchaToken:     req.CaptchaToken,
		// CloudFront geographic data
		Country:      cfInfo.Country,
		Region:       cfInfo.Region,
		RegionCode:   cfInfo.RegionCode,
		PostalCode:   cfInfo.PostalCode,
		UserTimezone: cfInfo.UserTimezone,
		Latitude:     cfInfo.Latitude,
		Longitude:    cfInfo.Longitude,
		MetroCode:    cfInfo.MetroCode,
		// CloudFront device detection (enums)
		DeviceType: cfInfo.DeviceType,
		DeviceOS:   cfInfo.DeviceOS,
	}

	response, err := h.processor.SignupUser(ctx, campaignID, processorReq, h.baseURL)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, response)
}

// HandleVerifyEmail handles GET /api/v1/campaigns/:campaign_id/verify (public endpoint)
// Verifies a user's email using the token from the verification email
func (h *Handler) HandleVerifyEmail(c *gin.Context) {
	ctx := c.Request.Context()

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	// Get token from query parameter
	token := c.Query("token")
	if token == "" {
		apierrors.BadRequest(c, "MISSING_TOKEN", "Verification token is required")
		return
	}

	err = h.processor.VerifyUserByToken(ctx, campaignID, token)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Email verified successfully",
	})
}

// HandleListUsers handles GET /api/v1/campaigns/:campaign_id/users
// Supports query parameters:
// - page, limit: pagination
// - status[]: multiple status values (e.g., status[]=pending&status[]=verified)
// - source[]: multiple source values
// - has_referrals: boolean
// - min_position, max_position: position range
// - date_from, date_to: ISO date strings
// - custom_fields[field_name]: custom field filters (e.g., custom_fields[company]=Acme)
// - sort, order: sorting
func (h *Handler) HandleListUsers(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "Account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		apierrors.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	// Parse pagination parameters
	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if _, err := fmt.Sscanf(pageStr, "%d", &page); err != nil || page < 1 {
			page = 1
		}
	}

	limit := 25
	if limitStr := c.Query("limit"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || limit < 1 {
			limit = 25
		}
		// Cap at 10000 for export use cases
		if limit > 10000 {
			limit = 10000
		}
	}

	// Parse status array (supports both status[] and status)
	var statuses []string
	if statusArray := c.QueryArray("status[]"); len(statusArray) > 0 {
		statuses = statusArray
	} else if statusStr := c.Query("status"); statusStr != "" {
		statuses = []string{statusStr}
	}

	// Parse source array
	var sources []string
	if sourceArray := c.QueryArray("source[]"); len(sourceArray) > 0 {
		sources = sourceArray
	} else if sourceStr := c.Query("source"); sourceStr != "" {
		sources = []string{sourceStr}
	}

	// Parse has_referrals boolean
	var hasReferrals *bool
	if hasReferralsStr := c.Query("has_referrals"); hasReferralsStr != "" {
		v := hasReferralsStr == "true"
		hasReferrals = &v
	}

	// Parse position range
	var minPosition, maxPosition *int
	if minPosStr := c.Query("min_position"); minPosStr != "" {
		var v int
		if _, err := fmt.Sscanf(minPosStr, "%d", &v); err == nil {
			minPosition = &v
		}
	}
	if maxPosStr := c.Query("max_position"); maxPosStr != "" {
		var v int
		if _, err := fmt.Sscanf(maxPosStr, "%d", &v); err == nil {
			maxPosition = &v
		}
	}

	// Parse date range
	var dateFrom, dateTo *string
	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		dateFrom = &dateFromStr
	}
	if dateToStr := c.Query("date_to"); dateToStr != "" {
		dateTo = &dateToStr
	}

	// Parse custom field filters (custom_fields[field_name]=value)
	customFields := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if strings.HasPrefix(key, "custom_fields[") && strings.HasSuffix(key, "]") {
			// Extract field name from custom_fields[field_name]
			fieldName := key[14 : len(key)-1] // Remove "custom_fields[" and "]"
			if len(values) > 0 && fieldName != "" {
				customFields[fieldName] = values[0]
			}
		}
	}

	sortBy := c.DefaultQuery("sort", "position")
	sortOrder := c.DefaultQuery("order", "asc")

	req := processor.ListUsersRequest{
		Statuses:     statuses,
		Sources:      sources,
		HasReferrals: hasReferrals,
		MinPosition:  minPosition,
		MaxPosition:  maxPosition,
		DateFrom:     dateFrom,
		DateTo:       dateTo,
		CustomFields: customFields,
		SortBy:       sortBy,
		SortOrder:    sortOrder,
		Page:         page,
		Limit:        limit,
	}

	response, err := h.processor.ListUsers(ctx, accountID, campaignID, req)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleGetUser handles GET /api/v1/campaigns/:campaign_id/users/:user_id
func (h *Handler) HandleGetUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "Account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		apierrors.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		apierrors.BadRequest(c, "INVALID_USER_ID", "Invalid user ID")
		return
	}

	user, err := h.processor.GetUser(ctx, accountID, campaignID, userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateUserRequest represents the HTTP request for updating a user
type UpdateUserRequest struct {
	FirstName *string                `json:"first_name,omitempty"`
	LastName  *string                `json:"last_name,omitempty"`
	Status    *string                `json:"status,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// HandleUpdateUser handles PUT /api/v1/campaigns/:campaign_id/users/:user_id
func (h *Handler) HandleUpdateUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "Account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		apierrors.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		apierrors.BadRequest(c, "INVALID_USER_ID", "Invalid user ID")
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	processorReq := processor.UpdateUserRequest{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Status:    req.Status,
		Metadata:  req.Metadata,
	}

	user, err := h.processor.UpdateUser(ctx, accountID, campaignID, userID, processorReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

// HandleDeleteUser handles DELETE /api/v1/campaigns/:campaign_id/users/:user_id
func (h *Handler) HandleDeleteUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "Account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		apierrors.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		apierrors.BadRequest(c, "INVALID_USER_ID", "Invalid user ID")
		return
	}

	err = h.processor.DeleteUser(ctx, accountID, campaignID, userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// SearchUsersRequest represents the HTTP request for searching users
type SearchUsersRequest struct {
	Query        *string  `json:"query,omitempty"`
	Status       []string `json:"status,omitempty"`
	Verified     *bool    `json:"verified,omitempty"`
	MinReferrals *int     `json:"min_referrals,omitempty"`
	DateFrom     *string  `json:"date_from,omitempty"`
	DateTo       *string  `json:"date_to,omitempty"`
}

// HandleSearchUsers handles POST /api/v1/campaigns/:campaign_id/users/search
func (h *Handler) HandleSearchUsers(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "Account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		apierrors.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	var req SearchUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	// Parse pagination from query params
	page := 1
	if pageStr := c.Query("page"); pageStr != "" {
		if _, err := fmt.Sscanf(pageStr, "%d", &page); err != nil || page < 1 {
			page = 1
		}
	}

	limit := 25
	if limitStr := c.Query("limit"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil || limit < 1 {
			limit = 25
		}
		if limit > 10000 {
			limit = 10000
		}
	}

	processorReq := processor.SearchUsersRequest{
		Query:        req.Query,
		Statuses:     req.Status,
		Verified:     req.Verified,
		MinReferrals: req.MinReferrals,
		DateFrom:     req.DateFrom,
		DateTo:       req.DateTo,
		Page:         page,
		Limit:        limit,
	}

	response, err := h.processor.SearchUsers(ctx, accountID, campaignID, processorReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// HandleImportUsers handles POST /api/v1/campaigns/:campaign_id/users/import
func (h *Handler) HandleImportUsers(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "Account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		apierrors.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	// Verify campaign ownership
	if err := h.processor.VerifyCampaignOwnership(ctx, accountID, campaignID); err != nil {
		h.handleError(c, err)
		return
	}

	// Check leads limit before starting import (per campaign)
	limitResult, err := h.processor.CheckLeadsLimit(ctx, accountID, campaignID, 0)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// If at limit, reject the import immediately
	if limitResult.Limit != nil && limitResult.CanAdd == 0 {
		apierrors.Forbidden(c, "LEADS_LIMIT_REACHED", "This campaign has reached its leads limit. Please upgrade your plan to add more leads.")
		return
	}

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.logger.Error(ctx, "failed to get file from request", err)
		apierrors.BadRequest(c, "FILE_REQUIRED", "File is required")
		return
	}
	defer file.Close()

	format := c.DefaultPostForm("format", "csv")

	// For now, return job accepted with a placeholder
	// In production, this would queue a background job
	jobID := uuid.New()

	// Determine max leads to import based on remaining limit
	maxToImport := -1 // -1 means unlimited
	if limitResult.Limit != nil {
		maxToImport = limitResult.CanAdd
	}

	// Parse the file based on format (simplified implementation)
	importedCount := 0
	if format == "csv" {
		importedCount, err = h.importFromCSV(ctx, campaignID, file, header, maxToImport)
	} else if format == "json" {
		importedCount, err = h.importFromJSON(ctx, accountID, campaignID, file, maxToImport)
	} else {
		apierrors.BadRequest(c, "INVALID_FORMAT", "Invalid format, must be csv or json")
		return
	}

	if err != nil {
		h.handleError(c, err)
		return
	}

	// Build response message
	message := fmt.Sprintf("Import completed successfully. Imported %d users.", importedCount)
	if limitResult.Limit != nil && importedCount < maxToImport {
		// Only show partial message if we actually hit the limit during import
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":         jobID,
		"message":        message,
		"status":         "completed",
		"imported_count": importedCount,
	})
}

// importFromCSV imports users from CSV file
func (h *Handler) importFromCSV(ctx context.Context, campaignID uuid.UUID, file multipart.File, header *multipart.FileHeader, maxToImport int) (int, error) {
	reader := csv.NewReader(file)

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("failed to read CSV headers: %w", err)
	}

	// Find column indices
	emailIdx := -1
	firstNameIdx := -1
	lastNameIdx := -1

	for i, h := range headers {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "email":
			emailIdx = i
		case "first_name", "firstname":
			firstNameIdx = i
		case "last_name", "lastname":
			lastNameIdx = i
		}
	}

	if emailIdx == -1 {
		return 0, fmt.Errorf("email column is required")
	}

	count := 0
	for {
		// Check if we've reached the import limit
		if maxToImport >= 0 && count >= maxToImport {
			h.logger.Info(ctx, "reached leads import limit")
			break
		}

		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			h.logger.Error(ctx, "error reading CSV row", err)
			continue
		}

		if len(record) <= emailIdx {
			continue
		}

		email := strings.TrimSpace(record[emailIdx])
		if email == "" {
			continue
		}

		var firstName, lastName *string
		if firstNameIdx != -1 && len(record) > firstNameIdx && record[firstNameIdx] != "" {
			fn := strings.TrimSpace(record[firstNameIdx])
			firstName = &fn
		}
		if lastNameIdx != -1 && len(record) > lastNameIdx && record[lastNameIdx] != "" {
			ln := strings.TrimSpace(record[lastNameIdx])
			lastName = &ln
		}

		// Create signup request with custom fields for first_name and last_name if present
		customFields := make(map[string]string)
		if firstName != nil && *firstName != "" {
			customFields["first_name"] = *firstName
		}
		if lastName != nil && *lastName != "" {
			customFields["last_name"] = *lastName
		}

		req := processor.SignupUserRequest{
			Email:         email,
			FirstName:     nil,
			LastName:      nil,
			CustomFields:  customFields,
			TermsAccepted: true,
		}

		_, err = h.processor.SignupUser(ctx, campaignID, req, h.baseURL)
		if err != nil {
			h.logger.Error(ctx, "failed to import user", err)
			continue
		}

		count++
	}

	return count, nil
}

// importFromJSON imports users from JSON file
func (h *Handler) importFromJSON(ctx context.Context, accountID, campaignID uuid.UUID, file multipart.File, maxToImport int) (int, error) {
	var users []SignupRequest
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&users); err != nil {
		return 0, fmt.Errorf("failed to decode JSON: %w", err)
	}

	count := 0
	for _, user := range users {
		// Check if we've reached the import limit
		if maxToImport >= 0 && count >= maxToImport {
			h.logger.Info(ctx, "reached leads import limit")
			break
		}

		req := processor.SignupUserRequest{
			Email:         user.Email,
			FirstName:     nil,
			LastName:      nil,
			CustomFields:  user.CustomFields,
			TermsAccepted: user.TermsAccepted,
		}

		_, err := h.processor.SignupUser(ctx, campaignID, req, h.baseURL)
		if err != nil {
			h.logger.Error(ctx, "failed to import user", err)
			continue
		}

		count++
	}

	return count, nil
}

// ExportUsersRequest represents the HTTP request for exporting users
type ExportUsersRequest struct {
	Format  string                 `json:"format"`
	Filters map[string]interface{} `json:"filters,omitempty"`
}

// HandleExportUsers handles POST /api/v1/campaigns/:campaign_id/users/export
func (h *Handler) HandleExportUsers(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "Account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		apierrors.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	var req ExportUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.ValidationError(c, err)
		return
	}

	if req.Format == "" {
		req.Format = "csv"
	}

	// Check if JSON export is requested and if account has the feature
	if req.Format == "json" {
		if err := h.processor.CheckJSONExportFeature(ctx, accountID); err != nil {
			h.handleError(c, err)
			return
		}
	}

	// For now, return job accepted with a placeholder download URL
	// In production, this would queue a background job and generate the file asynchronously
	jobID := uuid.New()
	downloadURL := fmt.Sprintf("%s/api/v1/exports/%s/download", h.baseURL, jobID)

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":       jobID,
		"campaign_id":  campaignID,
		"account_id":   accountID,
		"message":      "Export job created successfully",
		"download_url": downloadURL,
		"format":       req.Format,
	})
}

// HandleVerifyUser handles POST /api/v1/campaigns/:campaign_id/users/:user_id/verify
func (h *Handler) HandleVerifyUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "Account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		apierrors.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		apierrors.BadRequest(c, "INVALID_USER_ID", "Invalid user ID")
		return
	}

	err = h.processor.VerifyUser(ctx, accountID, campaignID, userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User verified successfully",
	})
}

// HandleResendVerification handles POST /api/v1/campaigns/:campaign_id/users/:user_id/resend-verification
func (h *Handler) HandleResendVerification(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "Account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		apierrors.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		apierrors.BadRequest(c, "INVALID_USER_ID", "Invalid user ID")
		return
	}

	_, err = h.processor.ResendVerificationToken(ctx, accountID, campaignID, userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Verification email sent successfully",
		"sent_at": strconv.FormatInt(c.Request.Context().Value("timestamp").(int64), 10),
	})
}

// HandleRecalculatePositions handles POST /api/v1/campaigns/:campaign_id/positions/recalculate
// Admin endpoint to manually trigger position recalculation for a campaign
func (h *Handler) HandleRecalculatePositions(c *gin.Context) {
	ctx := c.Request.Context()

	// Get account ID from context
	accountIDStr, exists := c.Get("Account-ID")
	if !exists {
		apierrors.Unauthorized(c, "Account ID not found in context")
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		apierrors.BadRequest(c, "INVALID_ACCOUNT_ID", "Invalid account ID")
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		apierrors.BadRequest(c, "INVALID_CAMPAIGN_ID", "Invalid campaign ID")
		return
	}

	// Verify campaign ownership
	err = h.processor.VerifyCampaignOwnership(ctx, accountID, campaignID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	// Trigger position recalculation
	err = h.positionCalculator.CalculatePositionsForCampaign(ctx, campaignID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Positions recalculated successfully",
		"campaign_id": campaignID,
	})
}
