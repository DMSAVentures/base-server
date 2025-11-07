package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"base-server/internal/jobs"
	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// FraudWorker handles fraud detection jobs
type FraudWorker struct {
	store  *store.Store
	logger *observability.Logger
}

// NewFraudWorker creates a new fraud worker
func NewFraudWorker(store *store.Store, logger *observability.Logger) *FraudWorker {
	return &FraudWorker{
		store:  store,
		logger: logger,
	}
}

// ProcessFraudDetection processes a fraud detection job (for Kafka)
func (w *FraudWorker) ProcessFraudDetection(ctx context.Context, payload jobs.FraudDetectionJobPayload) error {
	return w.processFraudDetection(ctx, payload)
}

// ProcessFraudDetectionTask processes a fraud detection task (for Asynq)
func (w *FraudWorker) ProcessFraudDetectionTask(ctx context.Context, task *asynq.Task) error {
	var payload jobs.FraudDetectionJobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		w.logger.Error(ctx, "failed to unmarshal fraud detection job payload", err)
		return fmt.Errorf("failed to unmarshal fraud detection job payload: %w", err)
	}
	return w.processFraudDetection(ctx, payload)
}

// processFraudDetection contains the core fraud detection logic
func (w *FraudWorker) processFraudDetection(ctx context.Context, payload jobs.FraudDetectionJobPayload) error {
	// Get user
	user, err := w.store.GetWaitlistUserByID(ctx, payload.UserID)
	if err != nil {
		w.logger.Error(ctx, "failed to get user", err)
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Run fraud checks based on check types
	var detections []store.FraudDetection

	for _, checkType := range payload.CheckTypes {
		var detection *store.FraudDetection
		var err error

		switch checkType {
		case "self_referral":
			detection, err = w.checkSelfReferral(ctx, user)
		case "velocity":
			detection, err = w.checkVelocity(ctx, user)
		case "fake_email":
			detection, err = w.checkFakeEmail(ctx, user)
		case "bot":
			detection, err = w.checkBotBehavior(ctx, user)
		case "duplicate_ip":
			detection, err = w.checkDuplicateIP(ctx, user)
		case "duplicate_device":
			detection, err = w.checkDuplicateDevice(ctx, user)
		default:
			w.logger.Error(ctx, fmt.Sprintf("unknown fraud check type: %s", checkType), nil)
			continue
		}

		if err != nil {
			w.logger.Error(ctx, fmt.Sprintf("failed to run fraud check %s", checkType), err)
			continue
		}

		if detection != nil {
			detections = append(detections, *detection)
		}
	}

	// If no specific check types provided, run all checks
	if len(payload.CheckTypes) == 0 {
		allDetections, err := w.runAllChecks(ctx, user)
		if err != nil {
			w.logger.Error(ctx, "failed to run all fraud checks", err)
			return fmt.Errorf("failed to run all fraud checks: %w", err)
		}
		detections = allDetections
	}

	w.logger.Info(ctx, fmt.Sprintf("completed fraud detection for user %s, found %d potential issues", user.Email, len(detections)))
	return nil
}

// checkSelfReferral checks if a user referred themselves
func (w *FraudWorker) checkSelfReferral(ctx context.Context, user store.WaitlistUser) (*store.FraudDetection, error) {
	// If user has no referrer, skip
	if user.ReferredByID == nil {
		return nil, nil
	}

	// Get referrer
	referrer, err := w.store.GetWaitlistUserByID(ctx, *user.ReferredByID)
	if err != nil {
		return nil, fmt.Errorf("failed to get referrer: %w", err)
	}

	// Check if they share the same IP address
	if user.IPAddress != nil && referrer.IPAddress != nil && *user.IPAddress == *referrer.IPAddress {
		detection, err := w.store.CreateFraudDetection(ctx, store.CreateFraudDetectionParams{
			CampaignID:      user.CampaignID,
			UserID:          &user.ID,
			DetectionType:   "self_referral",
			ConfidenceScore: 0.90,
			Details: map[string]interface{}{
				"reason":      "Referred user and referrer share the same IP address",
				"referrer_id": referrer.ID,
				"ip_address":  *user.IPAddress,
			},
		})
		if err != nil {
			return nil, err
		}
		return &detection, nil
	}

	// Check if they share the same device fingerprint
	if user.DeviceFingerprint != nil && referrer.DeviceFingerprint != nil && *user.DeviceFingerprint == *referrer.DeviceFingerprint {
		detection, err := w.store.CreateFraudDetection(ctx, store.CreateFraudDetectionParams{
			CampaignID:      user.CampaignID,
			UserID:          &user.ID,
			DetectionType:   "self_referral",
			ConfidenceScore: 0.95,
			Details: map[string]interface{}{
				"reason":            "Referred user and referrer share the same device fingerprint",
				"referrer_id":       referrer.ID,
				"device_fingerprint": *user.DeviceFingerprint,
			},
		})
		if err != nil {
			return nil, err
		}
		return &detection, nil
	}

	return nil, nil
}

// checkVelocity checks if a user is making too many referrals too quickly
func (w *FraudWorker) checkVelocity(ctx context.Context, user store.WaitlistUser) (*store.FraudDetection, error) {
	// Count recent referrals (last hour)
	count, err := w.store.CountRecentReferralsByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to count recent referrals: %w", err)
	}

	// If more than 10 referrals in the last hour, flag as suspicious
	if count > 10 {
		detection, err := w.store.CreateFraudDetection(ctx, store.CreateFraudDetectionParams{
			CampaignID:      user.CampaignID,
			UserID:          &user.ID,
			DetectionType:   "velocity",
			ConfidenceScore: 0.80,
			Details: map[string]interface{}{
				"reason":           "Too many referrals in a short time period",
				"referrals_count":  count,
				"time_period":      "1 hour",
			},
		})
		if err != nil {
			return nil, err
		}
		return &detection, nil
	}

	return nil, nil
}

// checkFakeEmail checks if the email is from a disposable email provider
func (w *FraudWorker) checkFakeEmail(ctx context.Context, user store.WaitlistUser) (*store.FraudDetection, error) {
	// List of common disposable email domains
	disposableDomains := []string{
		"tempmail.com", "throwaway.email", "guerrillamail.com", "10minutemail.com",
		"mailinator.com", "yopmail.com", "temp-mail.org", "fakeinbox.com",
	}

	// Extract domain from email
	parts := strings.Split(user.Email, "@")
	if len(parts) != 2 {
		return nil, nil
	}
	domain := strings.ToLower(parts[1])

	// Check if domain is in disposable list
	for _, disposableDomain := range disposableDomains {
		if domain == disposableDomain {
			detection, err := w.store.CreateFraudDetection(ctx, store.CreateFraudDetectionParams{
				CampaignID:      user.CampaignID,
				UserID:          &user.ID,
				DetectionType:   "fake_email",
				ConfidenceScore: 0.85,
				Details: map[string]interface{}{
					"reason": "Email is from a disposable email provider",
					"domain": domain,
				},
			})
			if err != nil {
				return nil, err
			}
			return &detection, nil
		}
	}

	return nil, nil
}

// checkBotBehavior checks for bot-like behavior based on user agent
func (w *FraudWorker) checkBotBehavior(ctx context.Context, user store.WaitlistUser) (*store.FraudDetection, error) {
	if user.UserAgent == nil {
		return nil, nil
	}

	userAgent := strings.ToLower(*user.UserAgent)

	// List of common bot indicators
	botIndicators := []string{"bot", "crawler", "spider", "scraper", "curl", "wget", "python", "java/"}

	for _, indicator := range botIndicators {
		if strings.Contains(userAgent, indicator) {
			detection, err := w.store.CreateFraudDetection(ctx, store.CreateFraudDetectionParams{
				CampaignID:      user.CampaignID,
				UserID:          &user.ID,
				DetectionType:   "bot",
				ConfidenceScore: 0.90,
				Details: map[string]interface{}{
					"reason":     "User agent indicates automated/bot behavior",
					"user_agent": *user.UserAgent,
					"indicator":  indicator,
				},
			})
			if err != nil {
				return nil, err
			}
			return &detection, nil
		}
	}

	return nil, nil
}

// checkDuplicateIP checks if multiple users are using the same IP
func (w *FraudWorker) checkDuplicateIP(ctx context.Context, user store.WaitlistUser) (*store.FraudDetection, error) {
	if user.IPAddress == nil {
		return nil, nil
	}

	// Get users with same IP
	users, err := w.store.GetUsersByIPAddress(ctx, user.CampaignID, *user.IPAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by IP: %w", err)
	}

	// If more than 3 users share the same IP, flag as suspicious
	if len(users) > 3 {
		detection, err := w.store.CreateFraudDetection(ctx, store.CreateFraudDetectionParams{
			CampaignID:      user.CampaignID,
			UserID:          &user.ID,
			DetectionType:   "duplicate_ip",
			ConfidenceScore: 0.75,
			Details: map[string]interface{}{
				"reason":     "Multiple users sharing the same IP address",
				"ip_address": *user.IPAddress,
				"user_count": len(users),
			},
		})
		if err != nil {
			return nil, err
		}
		return &detection, nil
	}

	return nil, nil
}

// checkDuplicateDevice checks if multiple users are using the same device
func (w *FraudWorker) checkDuplicateDevice(ctx context.Context, user store.WaitlistUser) (*store.FraudDetection, error) {
	if user.DeviceFingerprint == nil {
		return nil, nil
	}

	// Get users with same device fingerprint
	users, err := w.store.GetUsersByDeviceFingerprint(ctx, user.CampaignID, *user.DeviceFingerprint)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by device fingerprint: %w", err)
	}

	// If more than 2 users share the same device, flag as suspicious
	if len(users) > 2 {
		detection, err := w.store.CreateFraudDetection(ctx, store.CreateFraudDetectionParams{
			CampaignID:      user.CampaignID,
			UserID:          &user.ID,
			DetectionType:   "duplicate_device",
			ConfidenceScore: 0.85,
			Details: map[string]interface{}{
				"reason":            "Multiple users sharing the same device fingerprint",
				"device_fingerprint": *user.DeviceFingerprint,
				"user_count":        len(users),
			},
		})
		if err != nil {
			return nil, err
		}
		return &detection, nil
	}

	return nil, nil
}

// runAllChecks runs all fraud detection checks
func (w *FraudWorker) runAllChecks(ctx context.Context, user store.WaitlistUser) ([]store.FraudDetection, error) {
	var detections []store.FraudDetection

	checks := []struct {
		name string
		fn   func(context.Context, store.WaitlistUser) (*store.FraudDetection, error)
	}{
		{"self_referral", w.checkSelfReferral},
		{"velocity", w.checkVelocity},
		{"fake_email", w.checkFakeEmail},
		{"bot", w.checkBotBehavior},
		{"duplicate_ip", w.checkDuplicateIP},
		{"duplicate_device", w.checkDuplicateDevice},
	}

	for _, check := range checks {
		detection, err := check.fn(ctx, user)
		if err != nil {
			w.logger.Error(ctx, fmt.Sprintf("failed to run %s check", check.name), err)
			continue
		}
		if detection != nil {
			detections = append(detections, *detection)
		}
	}

	return detections, nil
}
