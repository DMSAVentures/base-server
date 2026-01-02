package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"base-server/internal/tiers"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

// mockTierStore is a test implementation that returns unlimited tier info with all features enabled
type mockTierStore struct{}

func (m *mockTierStore) GetTierInfoByAccountID(ctx context.Context, accountID uuid.UUID) (store.TierInfo, error) {
	return store.TierInfo{
		PriceDescription: "team",
		Features:         map[string]bool{"visual_email_builder": true},
		Limits:           map[string]*int{},
	}, nil
}

func (m *mockTierStore) GetTierInfoByUserID(ctx context.Context, userID uuid.UUID) (store.TierInfo, error) {
	return m.GetTierInfoByAccountID(ctx, uuid.Nil)
}

func (m *mockTierStore) GetTierInfoByPriceID(ctx context.Context, priceID uuid.UUID) (store.TierInfo, error) {
	return m.GetTierInfoByAccountID(ctx, uuid.Nil)
}

func (m *mockTierStore) GetFreeTierInfo(ctx context.Context) (store.TierInfo, error) {
	return m.GetTierInfoByAccountID(ctx, uuid.Nil)
}

// createTestTierService creates a TierService with all features enabled for testing
func createTestTierService() *tiers.TierService {
	logger := observability.NewLogger()
	return tiers.New(&mockTierStore{}, logger)
}

func TestCreateCampaignEmailTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockCampaignEmailTemplateStore(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockEmailService, createTestTierService(), logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	templateID := uuid.New()

	t.Run("successfully creates campaign email template with defaults", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		expectedTemplate := store.CampaignEmailTemplate{
			ID:                templateID,
			CampaignID:        campaignID,
			Name:              "Welcome Email",
			Type:              "welcome",
			Subject:           "Welcome!",
			HTMLBody:          "<h1>Welcome</h1>",
			Enabled:           true,
			SendAutomatically: true,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			CreateCampaignEmailTemplate(gomock.Any(), store.CreateCampaignEmailTemplateParams{
				CampaignID:        campaignID,
				Name:              "Welcome Email",
				Type:              "welcome",
				Subject:           "Welcome!",
				HTMLBody:          "<h1>Welcome</h1>",
				Enabled:           true,
				SendAutomatically: true,
			}).
			Return(expectedTemplate, nil)

		req := CreateCampaignEmailTemplateRequest{
			Name:     "Welcome Email",
			Type:     "welcome",
			Subject:  "Welcome!",
			HTMLBody: "<h1>Welcome</h1>",
		}

		result, err := processor.CreateCampaignEmailTemplate(ctx, accountID, campaignID, req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.ID != templateID {
			t.Errorf("expected template ID %v, got %v", templateID, result.ID)
		}
	})

	t.Run("returns error for invalid template type", func(t *testing.T) {
		req := CreateCampaignEmailTemplateRequest{
			Name:     "Invalid Template",
			Type:     "invalid_type",
			Subject:  "Invalid",
			HTMLBody: "<h1>Invalid</h1>",
		}

		_, err := processor.CreateCampaignEmailTemplate(ctx, accountID, campaignID, req)

		if !errors.Is(err, ErrInvalidTemplateType) {
			t.Errorf("expected ErrInvalidTemplateType, got %v", err)
		}
	})

	t.Run("returns error for invalid HTML template content", func(t *testing.T) {
		req := CreateCampaignEmailTemplateRequest{
			Name:     "Invalid HTML",
			Type:     "welcome",
			Subject:  "Welcome",
			HTMLBody: "{{.Invalid",
		}

		_, err := processor.CreateCampaignEmailTemplate(ctx, accountID, campaignID, req)

		if !errors.Is(err, ErrInvalidTemplateContent) {
			t.Errorf("expected ErrInvalidTemplateContent, got %v", err)
		}
	})

	t.Run("returns error when campaign not found", func(t *testing.T) {
		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(store.Campaign{}, store.ErrNotFound)

		req := CreateCampaignEmailTemplateRequest{
			Name:     "Template",
			Type:     "welcome",
			Subject:  "Welcome",
			HTMLBody: "<h1>Welcome</h1>",
		}

		_, err := processor.CreateCampaignEmailTemplate(ctx, accountID, campaignID, req)

		if !errors.Is(err, ErrCampaignNotFound) {
			t.Errorf("expected ErrCampaignNotFound, got %v", err)
		}
	})

	t.Run("returns error when campaign belongs to different account", func(t *testing.T) {
		differentAccountID := uuid.New()
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: differentAccountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		req := CreateCampaignEmailTemplateRequest{
			Name:     "Template",
			Type:     "welcome",
			Subject:  "Welcome",
			HTMLBody: "<h1>Welcome</h1>",
		}

		_, err := processor.CreateCampaignEmailTemplate(ctx, accountID, campaignID, req)

		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}

func TestGetCampaignEmailTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockCampaignEmailTemplateStore(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockEmailService, createTestTierService(), logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	templateID := uuid.New()

	t.Run("successfully gets campaign email template", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		expectedTemplate := store.CampaignEmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       "Welcome Email",
			Type:       "welcome",
			Subject:    "Welcome!",
			HTMLBody:   "<h1>Welcome</h1>",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetCampaignEmailTemplateByID(gomock.Any(), templateID).
			Return(expectedTemplate, nil)

		result, err := processor.GetCampaignEmailTemplate(ctx, accountID, campaignID, templateID)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.ID != templateID {
			t.Errorf("expected template ID %v, got %v", templateID, result.ID)
		}
	})

	t.Run("returns error when campaign not found", func(t *testing.T) {
		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(store.Campaign{}, store.ErrNotFound)

		_, err := processor.GetCampaignEmailTemplate(ctx, accountID, campaignID, templateID)

		if !errors.Is(err, ErrCampaignNotFound) {
			t.Errorf("expected ErrCampaignNotFound, got %v", err)
		}
	})

	t.Run("returns error when template not found", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetCampaignEmailTemplateByID(gomock.Any(), templateID).
			Return(store.CampaignEmailTemplate{}, store.ErrNotFound)

		_, err := processor.GetCampaignEmailTemplate(ctx, accountID, campaignID, templateID)

		if !errors.Is(err, ErrCampaignEmailTemplateNotFound) {
			t.Errorf("expected ErrCampaignEmailTemplateNotFound, got %v", err)
		}
	})
}

func TestListCampaignEmailTemplates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockCampaignEmailTemplateStore(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockEmailService, createTestTierService(), logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	t.Run("successfully lists all campaign email templates", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		templates := []store.CampaignEmailTemplate{
			{
				ID:         uuid.New(),
				CampaignID: campaignID,
				Name:       "Welcome",
				Type:       "welcome",
			},
			{
				ID:         uuid.New(),
				CampaignID: campaignID,
				Name:       "Verification",
				Type:       "verification",
			},
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetCampaignEmailTemplatesByCampaign(gomock.Any(), campaignID).
			Return(templates, nil)

		result, err := processor.ListCampaignEmailTemplates(ctx, accountID, campaignID, nil)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 templates, got %d", len(result))
		}
	})

	t.Run("returns empty array when no templates exist", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetCampaignEmailTemplatesByCampaign(gomock.Any(), campaignID).
			Return(nil, nil)

		result, err := processor.ListCampaignEmailTemplates(ctx, accountID, campaignID, nil)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result == nil {
			t.Error("expected empty array, got nil")
		}
		if len(result) != 0 {
			t.Errorf("expected 0 templates, got %d", len(result))
		}
	})
}

func TestIsValidTemplateType(t *testing.T) {
	validTypes := []string{
		"verification",
		"welcome",
		"position_update",
		"reward_earned",
		"milestone",
		"custom",
	}

	for _, tt := range validTypes {
		t.Run("valid type: "+tt, func(t *testing.T) {
			if !isValidTemplateType(tt) {
				t.Errorf("expected %s to be valid", tt)
			}
		})
	}

	invalidTypes := []string{
		"invalid",
		"unknown",
		"",
		"WELCOME",
	}

	for _, tt := range invalidTypes {
		t.Run("invalid type: "+tt, func(t *testing.T) {
			if isValidTemplateType(tt) {
				t.Errorf("expected %s to be invalid", tt)
			}
		})
	}
}

func TestValidateTemplateContent(t *testing.T) {
	t.Run("valid template content", func(t *testing.T) {
		validContents := []string{
			"Hello World",
			"<h1>Hello {{.Name}}</h1>",
			"{{range .Items}}{{.}}{{end}}",
			"{{if .Condition}}Yes{{else}}No{{end}}",
		}

		for _, content := range validContents {
			err := validateTemplateContent(content)
			if err != nil {
				t.Errorf("expected content %q to be valid, got error: %v", content, err)
			}
		}
	})

	t.Run("invalid template content", func(t *testing.T) {
		invalidContents := []string{
			"{{.Invalid",
			"{{range}}",
			"{{if}}",
		}

		for _, content := range invalidContents {
			err := validateTemplateContent(content)
			if err == nil {
				t.Errorf("expected content %q to be invalid", content)
			}
		}
	})
}
