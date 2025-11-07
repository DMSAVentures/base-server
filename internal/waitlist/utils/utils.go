package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// GenerateReferralCode generates a unique referral code
func GenerateReferralCode(length int) (string, error) {
	if length <= 0 {
		length = 8
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to base64 and make URL-safe
	code := base64.URLEncoding.EncodeToString(bytes)
	// Remove padding and take first 'length' characters
	code = strings.TrimRight(code, "=")
	if len(code) > length {
		code = code[:length]
	}

	// Make uppercase for better readability
	return strings.ToUpper(code), nil
}

// GenerateVerificationToken generates a secure verification token
func GenerateVerificationToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate verification token: %w", err)
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}

// BuildReferralLink constructs a referral link with the given base URL and code
func BuildReferralLink(baseURL, campaignSlug, referralCode string) string {
	return fmt.Sprintf("%s/join/%s?ref=%s", baseURL, campaignSlug, referralCode)
}

// CalculatePositionChange calculates the new position after earning referrals
func CalculatePositionChange(currentPosition, referralCount, pointsPerReferral int) int {
	if referralCount <= 0 || pointsPerReferral <= 0 {
		return currentPosition
	}

	positionBoost := referralCount * pointsPerReferral
	newPosition := currentPosition - positionBoost

	// Position should never go below 1
	if newPosition < 1 {
		return 1
	}

	return newPosition
}

// IsVerificationTokenExpired checks if a verification token has expired
func IsVerificationTokenExpired(sentAt *time.Time, expiryHours int) bool {
	if sentAt == nil {
		return true
	}

	expiryTime := sentAt.Add(time.Duration(expiryHours) * time.Hour)
	return time.Now().After(expiryTime)
}
