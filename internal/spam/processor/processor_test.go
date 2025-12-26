package processor

import (
	"context"
	"errors"
	"testing"
	"time"

	"base-server/internal/observability"
	"base-server/internal/store"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()

	processor := New(mockStore, logger)

	if processor == nil {
		t.Fatal("expected non-nil processor")
	}
	if processor.store == nil {
		t.Fatal("expected non-nil store")
	}
	if processor.logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestAnalyzeSignup_NoDetections(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	user := store.WaitlistUser{
		ID:         uuid.New(),
		CampaignID: uuid.New(),
		Email:      "user@gmail.com",
	}

	// No referrer, no IP, clean email - no store calls expected except for velocity check
	// Since user.IPAddress is nil, velocity check is skipped
	// Since user.ReferredByID is nil, self-referral check is skipped
	// Email is not disposable, so no fraud detection is created

	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_DisposableEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	userID := uuid.New()
	campaignID := uuid.New()
	user := store.WaitlistUser{
		ID:         userID,
		CampaignID: campaignID,
		Email:      "test@mailinator.com",
	}

	// Expect fraud detection to be created for disposable email
	mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.CreateFraudDetectionParams) (store.FraudDetection, error) {
			if params.DetectionType != store.FraudDetectionTypeFakeEmail {
				t.Errorf("expected detection type %s, got %s", store.FraudDetectionTypeFakeEmail, params.DetectionType)
			}
			if params.ConfidenceScore != 0.95 {
				t.Errorf("expected confidence score 0.95, got %f", params.ConfidenceScore)
			}
			if params.CampaignID != campaignID {
				t.Errorf("expected campaign ID %s, got %s", campaignID, params.CampaignID)
			}
			return store.FraudDetection{}, nil
		})

	// Expect user to be blocked (confidence >= 0.90)
	mockStore.EXPECT().
		BlockWaitlistUser(gomock.Any(), userID).
		Return(nil)

	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_SelfReferral_SameIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	userID := uuid.New()
	referrerID := uuid.New()
	campaignID := uuid.New()
	ipAddress := "192.168.1.1"

	user := store.WaitlistUser{
		ID:           userID,
		CampaignID:   campaignID,
		Email:        "user@example.com",
		ReferredByID: &referrerID,
		IPAddress:    &ipAddress,
	}

	referrer := store.WaitlistUser{
		ID:        referrerID,
		Email:     "referrer@example.com",
		IPAddress: &ipAddress,
	}

	// Expect referrer lookup
	mockStore.EXPECT().
		GetWaitlistUserByID(gomock.Any(), referrerID).
		Return(referrer, nil)

	// Expect velocity check (user has IP address)
	mockStore.EXPECT().
		CountRecentSignupsByIP(gomock.Any(), campaignID, ipAddress, gomock.Any()).
		Return(1, nil) // Low count, no velocity detection

	// Expect fraud detection for self-referral (same IP = 0.95 confidence)
	mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.CreateFraudDetectionParams) (store.FraudDetection, error) {
			if params.DetectionType != store.FraudDetectionTypeSelfReferral {
				t.Errorf("expected detection type %s, got %s", store.FraudDetectionTypeSelfReferral, params.DetectionType)
			}
			if params.ConfidenceScore != 0.95 {
				t.Errorf("expected confidence score 0.95, got %f", params.ConfidenceScore)
			}
			return store.FraudDetection{}, nil
		})

	// Expect user to be blocked (confidence >= 0.90)
	mockStore.EXPECT().
		BlockWaitlistUser(gomock.Any(), userID).
		Return(nil)

	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_SelfReferral_SameDeviceFingerprint(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	userID := uuid.New()
	referrerID := uuid.New()
	campaignID := uuid.New()
	fingerprint := "device123abc"

	user := store.WaitlistUser{
		ID:                userID,
		CampaignID:        campaignID,
		Email:             "user@example.com",
		ReferredByID:      &referrerID,
		DeviceFingerprint: &fingerprint,
	}

	referrer := store.WaitlistUser{
		ID:                referrerID,
		Email:             "referrer@example.com",
		DeviceFingerprint: &fingerprint,
	}

	// Expect referrer lookup
	mockStore.EXPECT().
		GetWaitlistUserByID(gomock.Any(), referrerID).
		Return(referrer, nil)

	// Expect fraud detection for self-referral (same device fingerprint = 0.90 confidence)
	mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.CreateFraudDetectionParams) (store.FraudDetection, error) {
			if params.DetectionType != store.FraudDetectionTypeSelfReferral {
				t.Errorf("expected detection type %s, got %s", store.FraudDetectionTypeSelfReferral, params.DetectionType)
			}
			if params.ConfidenceScore != 0.90 {
				t.Errorf("expected confidence score 0.90, got %f", params.ConfidenceScore)
			}
			return store.FraudDetection{}, nil
		})

	// Expect user to be blocked (confidence >= 0.90)
	mockStore.EXPECT().
		BlockWaitlistUser(gomock.Any(), userID).
		Return(nil)

	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_SelfReferral_SimilarEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	userID := uuid.New()
	referrerID := uuid.New()
	campaignID := uuid.New()

	user := store.WaitlistUser{
		ID:           userID,
		CampaignID:   campaignID,
		Email:        "johndoe+test@gmail.com",
		ReferredByID: &referrerID,
	}

	referrer := store.WaitlistUser{
		ID:    referrerID,
		Email: "johndoe@gmail.com",
	}

	// Expect referrer lookup
	mockStore.EXPECT().
		GetWaitlistUserByID(gomock.Any(), referrerID).
		Return(referrer, nil)

	// Expect fraud detection for self-referral (email similarity = 0.70 confidence)
	mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.CreateFraudDetectionParams) (store.FraudDetection, error) {
			if params.DetectionType != store.FraudDetectionTypeSelfReferral {
				t.Errorf("expected detection type %s, got %s", store.FraudDetectionTypeSelfReferral, params.DetectionType)
			}
			if params.ConfidenceScore != 0.70 {
				t.Errorf("expected confidence score 0.70, got %f", params.ConfidenceScore)
			}
			return store.FraudDetection{}, nil
		})

	// No blocking expected (confidence < 0.90)

	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_VelocityDetection_HighThreshold(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	userID := uuid.New()
	campaignID := uuid.New()
	ipAddress := "192.168.1.1"

	user := store.WaitlistUser{
		ID:         userID,
		CampaignID: campaignID,
		Email:      "user@example.com",
		IPAddress:  &ipAddress,
	}

	// Expect velocity check - return high count (11 signups, after decrementing = 10)
	mockStore.EXPECT().
		CountRecentSignupsByIP(gomock.Any(), campaignID, ipAddress, gomock.Any()).
		Return(11, nil) // 11 - 1 = 10, which is >= velocityThresholdHigh (10)

	// Expect fraud detection for velocity (0.95 confidence)
	mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.CreateFraudDetectionParams) (store.FraudDetection, error) {
			if params.DetectionType != store.FraudDetectionTypeVelocity {
				t.Errorf("expected detection type %s, got %s", store.FraudDetectionTypeVelocity, params.DetectionType)
			}
			if params.ConfidenceScore != 0.95 {
				t.Errorf("expected confidence score 0.95, got %f", params.ConfidenceScore)
			}
			return store.FraudDetection{}, nil
		})

	// Expect user to be blocked (confidence >= 0.90)
	mockStore.EXPECT().
		BlockWaitlistUser(gomock.Any(), userID).
		Return(nil)

	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_VelocityDetection_MidThreshold(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	userID := uuid.New()
	campaignID := uuid.New()
	ipAddress := "192.168.1.1"

	user := store.WaitlistUser{
		ID:         userID,
		CampaignID: campaignID,
		Email:      "user@example.com",
		IPAddress:  &ipAddress,
	}

	// Expect velocity check - return mid count (7 signups, after decrementing = 6)
	mockStore.EXPECT().
		CountRecentSignupsByIP(gomock.Any(), campaignID, ipAddress, gomock.Any()).
		Return(7, nil) // 7 - 1 = 6, which is >= velocityThresholdMid (6)

	// Expect fraud detection for velocity (0.85 confidence)
	mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.CreateFraudDetectionParams) (store.FraudDetection, error) {
			if params.DetectionType != store.FraudDetectionTypeVelocity {
				t.Errorf("expected detection type %s, got %s", store.FraudDetectionTypeVelocity, params.DetectionType)
			}
			if params.ConfidenceScore != 0.85 {
				t.Errorf("expected confidence score 0.85, got %f", params.ConfidenceScore)
			}
			return store.FraudDetection{}, nil
		})

	// No blocking expected (confidence < 0.90)

	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_VelocityDetection_LowThreshold(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	userID := uuid.New()
	campaignID := uuid.New()
	ipAddress := "192.168.1.1"

	user := store.WaitlistUser{
		ID:         userID,
		CampaignID: campaignID,
		Email:      "user@example.com",
		IPAddress:  &ipAddress,
	}

	// Expect velocity check - return low count (4 signups, after decrementing = 3)
	mockStore.EXPECT().
		CountRecentSignupsByIP(gomock.Any(), campaignID, ipAddress, gomock.Any()).
		Return(4, nil) // 4 - 1 = 3, which is >= velocityThresholdLow (3)

	// Expect fraud detection for velocity (0.70 confidence)
	mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, params store.CreateFraudDetectionParams) (store.FraudDetection, error) {
			if params.DetectionType != store.FraudDetectionTypeVelocity {
				t.Errorf("expected detection type %s, got %s", store.FraudDetectionTypeVelocity, params.DetectionType)
			}
			if params.ConfidenceScore != 0.70 {
				t.Errorf("expected confidence score 0.70, got %f", params.ConfidenceScore)
			}
			return store.FraudDetection{}, nil
		})

	// No blocking expected (confidence < 0.90)

	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_MultipleDetections(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	userID := uuid.New()
	referrerID := uuid.New()
	campaignID := uuid.New()
	ipAddress := "192.168.1.1"

	user := store.WaitlistUser{
		ID:           userID,
		CampaignID:   campaignID,
		Email:        "test@mailinator.com", // disposable email
		ReferredByID: &referrerID,
		IPAddress:    &ipAddress,
	}

	referrer := store.WaitlistUser{
		ID:        referrerID,
		Email:     "referrer@example.com",
		IPAddress: &ipAddress, // same IP - self-referral
	}

	// Expect referrer lookup
	mockStore.EXPECT().
		GetWaitlistUserByID(gomock.Any(), referrerID).
		Return(referrer, nil)

	// Expect velocity check - high count
	mockStore.EXPECT().
		CountRecentSignupsByIP(gomock.Any(), campaignID, ipAddress, gomock.Any()).
		Return(15, nil) // Very high velocity

	// Expect 3 fraud detections to be created (self-referral, velocity, fake email)
	createFraudCalls := mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		Times(3).
		Return(store.FraudDetection{}, nil)
	_ = createFraudCalls

	// Expect user to be blocked 3 times (all detections have high confidence)
	mockStore.EXPECT().
		BlockWaitlistUser(gomock.Any(), userID).
		Times(3).
		Return(nil)

	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_SelfReferralCheckError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	referrerID := uuid.New()

	user := store.WaitlistUser{
		ID:           uuid.New(),
		CampaignID:   uuid.New(),
		Email:        "user@example.com",
		ReferredByID: &referrerID,
	}

	// Simulate error when fetching referrer
	mockStore.EXPECT().
		GetWaitlistUserByID(gomock.Any(), referrerID).
		Return(store.WaitlistUser{}, errors.New("database error"))

	// Analysis should continue despite the error (just logs it)
	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_VelocityCheckError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	ipAddress := "192.168.1.1"
	campaignID := uuid.New()

	user := store.WaitlistUser{
		ID:         uuid.New(),
		CampaignID: campaignID,
		Email:      "user@example.com",
		IPAddress:  &ipAddress,
	}

	// Simulate error when counting signups
	mockStore.EXPECT().
		CountRecentSignupsByIP(gomock.Any(), campaignID, ipAddress, gomock.Any()).
		Return(0, errors.New("database error"))

	// Analysis should continue despite the error (just logs it)
	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_CreateFraudDetectionError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	user := store.WaitlistUser{
		ID:         uuid.New(),
		CampaignID: uuid.New(),
		Email:      "test@mailinator.com",
	}

	// Simulate error when creating fraud detection
	mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		Return(store.FraudDetection{}, errors.New("database error"))

	// Analysis should continue despite the error (just logs it)
	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_BlockUserError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	userID := uuid.New()
	user := store.WaitlistUser{
		ID:         userID,
		CampaignID: uuid.New(),
		Email:      "test@mailinator.com",
	}

	// Fraud detection creation succeeds
	mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		Return(store.FraudDetection{}, nil)

	// Blocking fails
	mockStore.EXPECT().
		BlockWaitlistUser(gomock.Any(), userID).
		Return(errors.New("database error"))

	// Analysis should continue despite the error (just logs it)
	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckEmailSimilarity(t *testing.T) {
	logger := observability.NewLogger()
	processor := New(nil, logger)

	tests := []struct {
		name     string
		email1   string
		email2   string
		expected float64
	}{
		{
			name:     "same base email with plus alias",
			email1:   "user+test@gmail.com",
			email2:   "user@gmail.com",
			expected: 0.95,
		},
		{
			name:     "same base email with different plus aliases",
			email1:   "user+test1@gmail.com",
			email2:   "user+test2@gmail.com",
			expected: 0.95,
		},
		{
			name:     "same base with numeric suffix",
			email1:   "user1@gmail.com",
			email2:   "user2@gmail.com",
			expected: 0.85,
		},
		{
			name:     "same base with different numeric suffixes",
			email1:   "john123@gmail.com",
			email2:   "john456@gmail.com",
			expected: 0.85,
		},
		{
			name:     "different domains",
			email1:   "user@gmail.com",
			email2:   "user@yahoo.com",
			expected: 0,
		},
		{
			name:     "completely different emails",
			email1:   "alice@gmail.com",
			email2:   "bob@gmail.com",
			expected: 0,
		},
		{
			name:     "case insensitive",
			email1:   "USER@Gmail.com",
			email2:   "user@gmail.com",
			expected: 0.95,
		},
		{
			name:     "with whitespace",
			email1:   " user@gmail.com ",
			email2:   "user@gmail.com",
			expected: 0.95,
		},
		{
			name:     "invalid email missing domain",
			email1:   "user",
			email2:   "user@gmail.com",
			expected: 0,
		},
		{
			name:     "both invalid emails",
			email1:   "user",
			email2:   "other",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processor.checkEmailSimilarity(tt.email1, tt.email2)
			if result != tt.expected {
				t.Errorf("checkEmailSimilarity(%q, %q) = %f, expected %f", tt.email1, tt.email2, result, tt.expected)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected string
	}{
		{
			name:     "valid email",
			email:    "user@example.com",
			expected: "example.com",
		},
		{
			name:     "uppercase domain",
			email:    "user@EXAMPLE.COM",
			expected: "example.com",
		},
		{
			name:     "with whitespace",
			email:    " user@example.com ",
			expected: "example.com",
		},
		{
			name:     "subdomain",
			email:    "user@mail.example.com",
			expected: "mail.example.com",
		},
		{
			name:     "invalid email no at sign",
			email:    "userexample.com",
			expected: "",
		},
		{
			name:     "empty string",
			email:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDomain(tt.email)
			if result != tt.expected {
				t.Errorf("extractDomain(%q) = %q, expected %q", tt.email, result, tt.expected)
			}
		})
	}
}

func TestCheckDisposableEmail(t *testing.T) {
	logger := observability.NewLogger()
	processor := New(nil, logger)
	ctx := context.Background()

	tests := []struct {
		name           string
		email          string
		expectDetected bool
	}{
		{
			name:           "disposable mailinator",
			email:          "test@mailinator.com",
			expectDetected: true,
		},
		{
			name:           "disposable 10minutemail",
			email:          "test@10minutemail.com",
			expectDetected: true,
		},
		{
			name:           "disposable tempmail",
			email:          "test@tempmail.com",
			expectDetected: true,
		},
		{
			name:           "disposable guerrillamail",
			email:          "test@guerrillamail.com",
			expectDetected: true,
		},
		{
			name:           "legitimate gmail",
			email:          "test@gmail.com",
			expectDetected: false,
		},
		{
			name:           "legitimate yahoo",
			email:          "test@yahoo.com",
			expectDetected: false,
		},
		{
			name:           "legitimate corporate",
			email:          "test@company.com",
			expectDetected: false,
		},
		{
			name:           "invalid email",
			email:          "invalid",
			expectDetected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := store.WaitlistUser{
				ID:         uuid.New(),
				CampaignID: uuid.New(),
				Email:      tt.email,
			}

			result := processor.checkDisposableEmail(ctx, user)

			if tt.expectDetected && result == nil {
				t.Errorf("expected disposable email to be detected for %q", tt.email)
			}
			if !tt.expectDetected && result != nil {
				t.Errorf("expected no detection for %q, but got result", tt.email)
			}

			if result != nil {
				if result.DetectionType != store.FraudDetectionTypeFakeEmail {
					t.Errorf("expected detection type %s, got %s", store.FraudDetectionTypeFakeEmail, result.DetectionType)
				}
				if result.ConfidenceScore != 0.95 {
					t.Errorf("expected confidence score 0.95, got %f", result.ConfidenceScore)
				}
			}
		})
	}
}

func TestCheckSelfReferral_NoReferrer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)
	ctx := context.Background()

	user := store.WaitlistUser{
		ID:           uuid.New(),
		CampaignID:   uuid.New(),
		Email:        "user@example.com",
		ReferredByID: nil, // No referrer
	}

	// No store calls expected
	result, err := processor.checkSelfReferral(ctx, user)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected no result, got: %+v", result)
	}
}

func TestCheckSelfReferral_NoMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)
	ctx := context.Background()

	referrerID := uuid.New()
	userIP := "192.168.1.1"
	referrerIP := "10.0.0.1"

	user := store.WaitlistUser{
		ID:           uuid.New(),
		CampaignID:   uuid.New(),
		Email:        "user@example.com",
		ReferredByID: &referrerID,
		IPAddress:    &userIP,
	}

	referrer := store.WaitlistUser{
		ID:        referrerID,
		Email:     "different@other.com",
		IPAddress: &referrerIP,
	}

	mockStore.EXPECT().
		GetWaitlistUserByID(gomock.Any(), referrerID).
		Return(referrer, nil)

	result, err := processor.checkSelfReferral(ctx, user)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected no result for different users, got: %+v", result)
	}
}

func TestCheckVelocity_NoIPAddress(t *testing.T) {
	logger := observability.NewLogger()
	processor := New(nil, logger)
	ctx := context.Background()

	user := store.WaitlistUser{
		ID:         uuid.New(),
		CampaignID: uuid.New(),
		Email:      "user@example.com",
		IPAddress:  nil, // No IP address
	}

	// No store calls expected
	result, err := processor.checkVelocity(ctx, user)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected no result, got: %+v", result)
	}
}

func TestCheckVelocity_BelowThreshold(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)
	ctx := context.Background()

	ipAddress := "192.168.1.1"
	campaignID := uuid.New()

	user := store.WaitlistUser{
		ID:         uuid.New(),
		CampaignID: campaignID,
		Email:      "user@example.com",
		IPAddress:  &ipAddress,
	}

	// Return count of 2 (after decrement = 1, below threshold of 3)
	mockStore.EXPECT().
		CountRecentSignupsByIP(gomock.Any(), campaignID, ipAddress, gomock.Any()).
		Return(2, nil)

	result, err := processor.checkVelocity(ctx, user)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected no result for count below threshold, got: %+v", result)
	}
}

func TestProcessResult_BelowAutoBlockThreshold(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)
	ctx := context.Background()

	userID := uuid.New()
	campaignID := uuid.New()

	user := store.WaitlistUser{
		ID:         userID,
		CampaignID: campaignID,
		Email:      "user@example.com",
	}

	result := &FraudResult{
		DetectionType:   store.FraudDetectionTypeSelfReferral,
		ConfidenceScore: 0.70, // Below 0.90 threshold
		Details:         store.JSONB{"reason": "email_similarity"},
	}

	// Expect fraud detection to be created
	mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		Return(store.FraudDetection{}, nil)

	// NO BlockWaitlistUser call expected (confidence below threshold)

	err := processor.processResult(ctx, user, result)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestProcessResult_AboveAutoBlockThreshold(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)
	ctx := context.Background()

	userID := uuid.New()
	campaignID := uuid.New()

	user := store.WaitlistUser{
		ID:         userID,
		CampaignID: campaignID,
		Email:      "user@example.com",
	}

	result := &FraudResult{
		DetectionType:   store.FraudDetectionTypeSelfReferral,
		ConfidenceScore: 0.95, // Above 0.90 threshold
		Details:         store.JSONB{"reason": "same_ip"},
	}

	// Expect fraud detection to be created
	mockStore.EXPECT().
		CreateFraudDetection(gomock.Any(), gomock.Any()).
		Return(store.FraudDetection{}, nil)

	// Expect BlockWaitlistUser call (confidence above threshold)
	mockStore.EXPECT().
		BlockWaitlistUser(gomock.Any(), userID).
		Return(nil)

	err := processor.processResult(ctx, user, result)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestAnalyzeSignup_VelocityZeroCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)

	ctx := context.Background()
	ipAddress := "192.168.1.1"
	campaignID := uuid.New()

	user := store.WaitlistUser{
		ID:         uuid.New(),
		CampaignID: campaignID,
		Email:      "user@example.com",
		IPAddress:  &ipAddress,
	}

	// Return 0 count
	mockStore.EXPECT().
		CountRecentSignupsByIP(gomock.Any(), campaignID, ipAddress, gomock.Any()).
		Return(0, nil)

	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCheckSelfReferral_IPMatchPrioritized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)
	ctx := context.Background()

	referrerID := uuid.New()
	sharedIP := "192.168.1.1"
	sharedFingerprint := "device123"

	// Both IP and fingerprint match - should use IP (0.95) not fingerprint (0.90)
	user := store.WaitlistUser{
		ID:                uuid.New(),
		CampaignID:        uuid.New(),
		Email:             "user+test@gmail.com",
		ReferredByID:      &referrerID,
		IPAddress:         &sharedIP,
		DeviceFingerprint: &sharedFingerprint,
	}

	referrer := store.WaitlistUser{
		ID:                referrerID,
		Email:             "user@gmail.com", // Same base email
		IPAddress:         &sharedIP,
		DeviceFingerprint: &sharedFingerprint,
	}

	mockStore.EXPECT().
		GetWaitlistUserByID(gomock.Any(), referrerID).
		Return(referrer, nil)

	result, err := processor.checkSelfReferral(ctx, user)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	// IP match should be prioritized (0.95 confidence)
	if result.ConfidenceScore != 0.95 {
		t.Errorf("expected confidence score 0.95 for IP match, got %f", result.ConfidenceScore)
	}
	if result.Details["match_type"] != "ip_address" {
		t.Errorf("expected match_type ip_address, got %v", result.Details["match_type"])
	}
}

func TestCheckSelfReferral_FingerprintMatchWithoutIP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)
	ctx := context.Background()

	referrerID := uuid.New()
	sharedFingerprint := "device123"

	// Only fingerprint matches (no IP)
	user := store.WaitlistUser{
		ID:                uuid.New(),
		CampaignID:        uuid.New(),
		Email:             "user@example.com",
		ReferredByID:      &referrerID,
		DeviceFingerprint: &sharedFingerprint,
	}

	referrer := store.WaitlistUser{
		ID:                referrerID,
		Email:             "different@other.com",
		DeviceFingerprint: &sharedFingerprint,
	}

	mockStore.EXPECT().
		GetWaitlistUserByID(gomock.Any(), referrerID).
		Return(referrer, nil)

	result, err := processor.checkSelfReferral(ctx, user)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.ConfidenceScore != 0.90 {
		t.Errorf("expected confidence score 0.90 for fingerprint match, got %f", result.ConfidenceScore)
	}
	if result.Details["match_type"] != "device_fingerprint" {
		t.Errorf("expected match_type device_fingerprint, got %v", result.Details["match_type"])
	}
}

func TestVelocityWindow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockSpamStore(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, logger)
	ctx := context.Background()

	ipAddress := "192.168.1.1"
	campaignID := uuid.New()

	user := store.WaitlistUser{
		ID:         uuid.New(),
		CampaignID: campaignID,
		Email:      "user@example.com",
		IPAddress:  &ipAddress,
	}

	var capturedSince time.Time

	mockStore.EXPECT().
		CountRecentSignupsByIP(gomock.Any(), campaignID, ipAddress, gomock.Any()).
		DoAndReturn(func(ctx context.Context, campaignID uuid.UUID, ip string, since time.Time) (int, error) {
			capturedSince = since
			return 0, nil
		})

	err := processor.AnalyzeSignup(ctx, user)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify the since time is approximately 10 minutes ago
	expectedSince := time.Now().Add(-10 * time.Minute)
	diff := expectedSince.Sub(capturedSince)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("expected since time to be approximately 10 minutes ago, got diff: %v", diff)
	}
}
