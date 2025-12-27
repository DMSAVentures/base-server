package handler

import (
	"base-server/internal/apierrors"
	"base-server/internal/observability"
	"base-server/internal/waitlist/processor"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
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
		// CloudFront connection info
		ASN:         cfInfo.ASN,
		TLSVersion:  cfInfo.TLSVersion,
		HTTPVersion: cfInfo.HTTPVersion,
	}

	response, err := h.processor.SignupUser(ctx, campaignID, processorReq, h.baseURL)
	if err != nil {
		apierrors.RespondWithError(c, err)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get token from query parameter
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "verification token is required"})
		return
	}

	err = h.processor.VerifyUserByToken(ctx, campaignID, token)
	if err != nil {
		apierrors.RespondWithError(c, err)
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
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
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
		apierrors.RespondWithError(c, err)
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
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	user, err := h.processor.GetUser(ctx, accountID, campaignID, userID)
	if err != nil {
		apierrors.RespondWithError(c, err)
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
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
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
		apierrors.RespondWithError(c, err)
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
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	err = h.processor.DeleteUser(ctx, accountID, campaignID, userID)
	if err != nil {
		apierrors.RespondWithError(c, err)
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
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	var req SearchUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
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
		apierrors.RespondWithError(c, err)
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
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Parse multipart form
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.logger.Error(ctx, "failed to get file from request", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	format := c.DefaultPostForm("format", "csv")

	// For now, return job accepted with a placeholder
	// In production, this would queue a background job
	jobID := uuid.New()

	// Parse the file based on format (simplified implementation)
	importedCount := 0
	if format == "csv" {
		importedCount, err = h.importFromCSV(ctx, campaignID, file, header)
	} else if format == "json" {
		importedCount, err = h.importFromJSON(ctx, accountID, campaignID, file)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid format, must be csv or json"})
		return
	}

	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":         jobID,
		"message":        fmt.Sprintf("Import completed successfully. Imported %d users.", importedCount),
		"status":         "completed",
		"imported_count": importedCount,
	})
}

// importFromCSV imports users from CSV file
func (h *Handler) importFromCSV(ctx context.Context, campaignID uuid.UUID, file multipart.File, header *multipart.FileHeader) (int, error) {
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
func (h *Handler) importFromJSON(ctx context.Context, accountID, campaignID uuid.UUID, file multipart.File) (int, error) {
	var users []SignupRequest
	decoder := json.NewDecoder(file)

	if err := decoder.Decode(&users); err != nil {
		return 0, fmt.Errorf("failed to decode JSON: %w", err)
	}

	count := 0
	for _, user := range users {
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
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	var req ExportUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierrors.RespondWithValidationError(c, err)
		return
	}

	if req.Format == "" {
		req.Format = "csv"
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
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	err = h.processor.VerifyUser(ctx, accountID, campaignID, userID)
	if err != nil {
		apierrors.RespondWithError(c, err)
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
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Get user ID from path
	userIDStr := c.Param("user_id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse user ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	_, err = h.processor.ResendVerificationToken(ctx, accountID, campaignID, userID)
	if err != nil {
		apierrors.RespondWithError(c, err)
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
		apierrors.RespondWithError(c, apierrors.Unauthorized("account ID not found in context"))
		return
	}

	accountID, err := uuid.Parse(accountIDStr.(string))
	if err != nil {
		h.logger.Error(ctx, "failed to parse account ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	// Get campaign ID from path
	campaignIDStr := c.Param("campaign_id")
	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		h.logger.Error(ctx, "failed to parse campaign ID", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	// Verify campaign ownership
	err = h.processor.VerifyCampaignOwnership(ctx, accountID, campaignID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	// Trigger position recalculation
	err = h.positionCalculator.CalculatePositionsForCampaign(ctx, campaignID)
	if err != nil {
		apierrors.RespondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Positions recalculated successfully",
		"campaign_id": campaignID,
	})
}
