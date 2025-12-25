package processor

import (
	"context"
	"strings"
	"time"

	"base-server/internal/observability"
	"base-server/internal/spam"
	"base-server/internal/store"
)

const (
	// Confidence thresholds
	autoBlockThreshold = 0.90

	// Velocity detection settings
	velocityWindow        = 10 * time.Minute
	velocityThresholdLow  = 3  // 70% confidence
	velocityThresholdMid  = 6  // 85% confidence
	velocityThresholdHigh = 10 // 95% confidence
)

// FraudResult represents the result of a spam detection check
type FraudResult struct {
	DetectionType   string
	ConfidenceScore float64
	Details         store.JSONB
}

// Processor handles spam detection for waitlist signups
type Processor struct {
	store  SpamStore
	logger *observability.Logger
}

// New creates a new spam detection processor
func New(store SpamStore, logger *observability.Logger) *Processor {
	return &Processor{
		store:  store,
		logger: logger,
	}
}

// AnalyzeSignup runs all spam detection checks on a new signup
func (p *Processor) AnalyzeSignup(ctx context.Context, user store.WaitlistUser) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "user_id", Value: user.ID},
		observability.Field{Key: "campaign_id", Value: user.CampaignID},
		observability.Field{Key: "operation", Value: "spam_analysis"},
	)

	p.logger.Info(ctx, "Starting spam analysis for user")

	var results []*FraudResult

	// Check 1: Self-referral detection (highest priority)
	if result, err := p.checkSelfReferral(ctx, user); err != nil {
		p.logger.Error(ctx, "self-referral check failed", err)
	} else if result != nil {
		results = append(results, result)
	}

	// Check 2: Velocity detection
	if result, err := p.checkVelocity(ctx, user); err != nil {
		p.logger.Error(ctx, "velocity check failed", err)
	} else if result != nil {
		results = append(results, result)
	}

	// Check 3: Disposable email detection
	if result := p.checkDisposableEmail(ctx, user); result != nil {
		results = append(results, result)
	}

	// Process results
	for _, result := range results {
		if err := p.processResult(ctx, user, result); err != nil {
			p.logger.Error(ctx, "failed to process fraud result", err)
		}
	}

	if len(results) > 0 {
		ctx = observability.WithFields(ctx,
			observability.Field{Key: "detection_count", Value: len(results)})
		p.logger.Info(ctx, "Spam analysis completed with detections")
	} else {
		p.logger.Info(ctx, "Spam analysis completed - no issues detected")
	}

	return nil
}

// checkSelfReferral detects users referring themselves
func (p *Processor) checkSelfReferral(ctx context.Context, user store.WaitlistUser) (*FraudResult, error) {
	// Skip if user wasn't referred
	if user.ReferredByID == nil {
		return nil, nil
	}

	// Get the referrer
	referrer, err := p.store.GetWaitlistUserByID(ctx, *user.ReferredByID)
	if err != nil {
		return nil, err
	}

	var confidence float64
	details := store.JSONB{
		"referrer_id":    referrer.ID.String(),
		"referrer_email": referrer.Email,
		"user_email":     user.Email,
	}

	// Check 1: Same IP address (highest confidence)
	if user.IPAddress != nil && referrer.IPAddress != nil && *user.IPAddress == *referrer.IPAddress {
		confidence = 0.95
		details["match_type"] = "ip_address"
		details["ip_address"] = *user.IPAddress
	}

	// Check 2: Same device fingerprint
	if confidence == 0 && user.DeviceFingerprint != nil && referrer.DeviceFingerprint != nil &&
		*user.DeviceFingerprint == *referrer.DeviceFingerprint {
		confidence = 0.90
		details["match_type"] = "device_fingerprint"
	}

	// Check 3: Similar email pattern (e.g., user+1@gmail.com and user+2@gmail.com)
	if confidence == 0 {
		if similarity := p.checkEmailSimilarity(user.Email, referrer.Email); similarity > 0.8 {
			confidence = 0.70
			details["match_type"] = "email_similarity"
			details["similarity_score"] = similarity
		}
	}

	if confidence > 0 {
		return &FraudResult{
			DetectionType:   store.FraudDetectionTypeSelfReferral,
			ConfidenceScore: confidence,
			Details:         details,
		}, nil
	}

	return nil, nil
}

// checkEmailSimilarity compares two emails for suspicious similarity
func (p *Processor) checkEmailSimilarity(email1, email2 string) float64 {
	// Normalize emails
	e1 := strings.ToLower(strings.TrimSpace(email1))
	e2 := strings.ToLower(strings.TrimSpace(email2))

	// Extract local parts and domains
	parts1 := strings.SplitN(e1, "@", 2)
	parts2 := strings.SplitN(e2, "@", 2)

	if len(parts1) != 2 || len(parts2) != 2 {
		return 0
	}

	local1, domain1 := parts1[0], parts1[1]
	local2, domain2 := parts2[0], parts2[1]

	// Different domains = low similarity
	if domain1 != domain2 {
		return 0
	}

	// Remove + aliases (e.g., user+test@gmail.com -> user@gmail.com)
	baseLocal1 := strings.SplitN(local1, "+", 2)[0]
	baseLocal2 := strings.SplitN(local2, "+", 2)[0]

	// Same base local part = high similarity
	if baseLocal1 == baseLocal2 {
		return 0.95
	}

	// Check for numeric suffixes (user1, user2, etc.)
	base1 := strings.TrimRight(baseLocal1, "0123456789")
	base2 := strings.TrimRight(baseLocal2, "0123456789")

	if base1 == base2 && base1 != "" {
		return 0.85
	}

	return 0
}

// checkVelocity detects rapid signups from the same IP
func (p *Processor) checkVelocity(ctx context.Context, user store.WaitlistUser) (*FraudResult, error) {
	// Skip if no IP address
	if user.IPAddress == nil {
		return nil, nil
	}

	since := time.Now().Add(-velocityWindow)
	count, err := p.store.CountRecentSignupsByIP(ctx, user.CampaignID, *user.IPAddress, since)
	if err != nil {
		return nil, err
	}

	// Don't count the current user (they're already in the DB)
	if count > 0 {
		count--
	}

	var confidence float64
	if count >= velocityThresholdHigh {
		confidence = 0.95
	} else if count >= velocityThresholdMid {
		confidence = 0.85
	} else if count >= velocityThresholdLow {
		confidence = 0.70
	}

	if confidence > 0 {
		return &FraudResult{
			DetectionType:   store.FraudDetectionTypeVelocity,
			ConfidenceScore: confidence,
			Details: store.JSONB{
				"ip_address":     *user.IPAddress,
				"signup_count":   count,
				"window_minutes": int(velocityWindow.Minutes()),
			},
		}, nil
	}

	return nil, nil
}

// checkDisposableEmail checks if the email domain is a known disposable email provider
func (p *Processor) checkDisposableEmail(ctx context.Context, user store.WaitlistUser) *FraudResult {
	domain := extractDomain(user.Email)
	if domain == "" {
		return nil
	}

	if spam.IsDisposableDomain(domain) {
		return &FraudResult{
			DetectionType:   store.FraudDetectionTypeFakeEmail,
			ConfidenceScore: 0.95,
			Details: store.JSONB{
				"email":  user.Email,
				"domain": domain,
				"reason": "disposable_email_provider",
			},
		}
	}

	return nil
}

// processResult creates a fraud detection record and optionally blocks the user
func (p *Processor) processResult(ctx context.Context, user store.WaitlistUser, result *FraudResult) error {
	ctx = observability.WithFields(ctx,
		observability.Field{Key: "detection_type", Value: result.DetectionType},
		observability.Field{Key: "confidence_score", Value: result.ConfidenceScore},
	)

	// Create fraud detection record
	params := store.CreateFraudDetectionParams{
		CampaignID:      user.CampaignID,
		UserID:          &user.ID,
		DetectionType:   result.DetectionType,
		ConfidenceScore: result.ConfidenceScore,
		Details:         result.Details,
	}

	if _, err := p.store.CreateFraudDetection(ctx, params); err != nil {
		p.logger.Error(ctx, "failed to create fraud detection record", err)
		return err
	}

	p.logger.Info(ctx, "Created fraud detection record")

	// Auto-block if confidence is above threshold
	if result.ConfidenceScore >= autoBlockThreshold {
		if err := p.store.BlockWaitlistUser(ctx, user.ID); err != nil {
			p.logger.Error(ctx, "failed to block user", err)
			return err
		}
		p.logger.Info(ctx, "Auto-blocked user due to high confidence spam detection")
	}

	return nil
}

// extractDomain extracts the domain from an email address
func extractDomain(email string) string {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(email)), "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}
