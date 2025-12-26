package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestCreateEmailTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailTemplateStore(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockEmailService, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	templateID := uuid.New()

	t.Run("successfully creates email template with defaults", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		expectedTemplate := store.EmailTemplate{
			ID:                templateID,
			CampaignID:        campaignID,
			Name:              "Welcome Email",
			Type:              "welcome",
			Subject:           "Welcome!",
			HTMLBody:          "<h1>Welcome</h1>",
			TextBody:          "Welcome",
			Enabled:           true,
			SendAutomatically: true,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			CreateEmailTemplate(gomock.Any(), store.CreateEmailTemplateParams{
				CampaignID:        campaignID,
				Name:              "Welcome Email",
				Type:              "welcome",
				Subject:           "Welcome!",
				HTMLBody:          "<h1>Welcome</h1>",
				TextBody:          "Welcome",
				Enabled:           true,
				SendAutomatically: true,
			}).
			Return(expectedTemplate, nil)

		req := CreateEmailTemplateRequest{
			Name:     "Welcome Email",
			Type:     "welcome",
			Subject:  "Welcome!",
			HTMLBody: "<h1>Welcome</h1>",
			TextBody: "Welcome",
		}

		result, err := processor.CreateEmailTemplate(ctx, accountID, campaignID, req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.ID != templateID {
			t.Errorf("expected template ID %v, got %v", templateID, result.ID)
		}
	})

	t.Run("successfully creates email template with custom enabled and sendAutomatically", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		enabled := false
		sendAutomatically := false

		expectedTemplate := store.EmailTemplate{
			ID:                templateID,
			CampaignID:        campaignID,
			Name:              "Disabled Template",
			Type:              "custom",
			Subject:           "Custom",
			HTMLBody:          "<h1>Custom</h1>",
			TextBody:          "Custom",
			Enabled:           false,
			SendAutomatically: false,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			CreateEmailTemplate(gomock.Any(), store.CreateEmailTemplateParams{
				CampaignID:        campaignID,
				Name:              "Disabled Template",
				Type:              "custom",
				Subject:           "Custom",
				HTMLBody:          "<h1>Custom</h1>",
				TextBody:          "Custom",
				Enabled:           false,
				SendAutomatically: false,
			}).
			Return(expectedTemplate, nil)

		req := CreateEmailTemplateRequest{
			Name:              "Disabled Template",
			Type:              "custom",
			Subject:           "Custom",
			HTMLBody:          "<h1>Custom</h1>",
			TextBody:          "Custom",
			Enabled:           &enabled,
			SendAutomatically: &sendAutomatically,
		}

		result, err := processor.CreateEmailTemplate(ctx, accountID, campaignID, req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Enabled != false {
			t.Errorf("expected enabled to be false, got %v", result.Enabled)
		}
		if result.SendAutomatically != false {
			t.Errorf("expected sendAutomatically to be false, got %v", result.SendAutomatically)
		}
	})

	t.Run("returns error for invalid template type", func(t *testing.T) {
		req := CreateEmailTemplateRequest{
			Name:     "Invalid Template",
			Type:     "invalid_type",
			Subject:  "Invalid",
			HTMLBody: "<h1>Invalid</h1>",
			TextBody: "Invalid",
		}

		_, err := processor.CreateEmailTemplate(ctx, accountID, campaignID, req)

		if !errors.Is(err, ErrInvalidTemplateType) {
			t.Errorf("expected ErrInvalidTemplateType, got %v", err)
		}
	})

	t.Run("returns error for invalid HTML template content", func(t *testing.T) {
		req := CreateEmailTemplateRequest{
			Name:     "Invalid HTML",
			Type:     "welcome",
			Subject:  "Welcome",
			HTMLBody: "{{.Invalid",
			TextBody: "Valid",
		}

		_, err := processor.CreateEmailTemplate(ctx, accountID, campaignID, req)

		if !errors.Is(err, ErrInvalidTemplateContent) {
			t.Errorf("expected ErrInvalidTemplateContent, got %v", err)
		}
	})

	t.Run("returns error for invalid text template content", func(t *testing.T) {
		req := CreateEmailTemplateRequest{
			Name:     "Invalid Text",
			Type:     "welcome",
			Subject:  "Welcome",
			HTMLBody: "<h1>Valid</h1>",
			TextBody: "{{.Invalid",
		}

		_, err := processor.CreateEmailTemplate(ctx, accountID, campaignID, req)

		if !errors.Is(err, ErrInvalidTemplateContent) {
			t.Errorf("expected ErrInvalidTemplateContent, got %v", err)
		}
	})

	t.Run("returns error when campaign not found", func(t *testing.T) {
		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(store.Campaign{}, store.ErrNotFound)

		req := CreateEmailTemplateRequest{
			Name:     "Template",
			Type:     "welcome",
			Subject:  "Welcome",
			HTMLBody: "<h1>Welcome</h1>",
			TextBody: "Welcome",
		}

		_, err := processor.CreateEmailTemplate(ctx, accountID, campaignID, req)

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

		req := CreateEmailTemplateRequest{
			Name:     "Template",
			Type:     "welcome",
			Subject:  "Welcome",
			HTMLBody: "<h1>Welcome</h1>",
			TextBody: "Welcome",
		}

		_, err := processor.CreateEmailTemplate(ctx, accountID, campaignID, req)

		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("returns error when store fails to create template", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		storeErr := errors.New("database error")

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			CreateEmailTemplate(gomock.Any(), gomock.Any()).
			Return(store.EmailTemplate{}, storeErr)

		req := CreateEmailTemplateRequest{
			Name:     "Template",
			Type:     "welcome",
			Subject:  "Welcome",
			HTMLBody: "<h1>Welcome</h1>",
			TextBody: "Welcome",
		}

		_, err := processor.CreateEmailTemplate(ctx, accountID, campaignID, req)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestGetEmailTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailTemplateStore(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockEmailService, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	templateID := uuid.New()

	t.Run("successfully gets email template", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		expectedTemplate := store.EmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       "Welcome Email",
			Type:       "welcome",
			Subject:    "Welcome!",
			HTMLBody:   "<h1>Welcome</h1>",
			TextBody:   "Welcome",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(expectedTemplate, nil)

		result, err := processor.GetEmailTemplate(ctx, accountID, campaignID, templateID)

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

		_, err := processor.GetEmailTemplate(ctx, accountID, campaignID, templateID)

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

		_, err := processor.GetEmailTemplate(ctx, accountID, campaignID, templateID)

		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
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
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(store.EmailTemplate{}, store.ErrNotFound)

		_, err := processor.GetEmailTemplate(ctx, accountID, campaignID, templateID)

		if !errors.Is(err, ErrTemplateNotFound) {
			t.Errorf("expected ErrTemplateNotFound, got %v", err)
		}
	})

	t.Run("returns error when template belongs to different campaign", func(t *testing.T) {
		differentCampaignID := uuid.New()
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		template := store.EmailTemplate{
			ID:         templateID,
			CampaignID: differentCampaignID,
			Name:       "Welcome Email",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(template, nil)

		_, err := processor.GetEmailTemplate(ctx, accountID, campaignID, templateID)

		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}

func TestListEmailTemplates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailTemplateStore(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockEmailService, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()

	t.Run("successfully lists all email templates", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		templates := []store.EmailTemplate{
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
			GetEmailTemplatesByCampaign(gomock.Any(), campaignID).
			Return(templates, nil)

		result, err := processor.ListEmailTemplates(ctx, accountID, campaignID, nil)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 templates, got %d", len(result))
		}
	})

	t.Run("successfully filters templates by type", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		templates := []store.EmailTemplate{
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
			GetEmailTemplatesByCampaign(gomock.Any(), campaignID).
			Return(templates, nil)

		templateType := "welcome"
		result, err := processor.ListEmailTemplates(ctx, accountID, campaignID, &templateType)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("expected 1 template, got %d", len(result))
		}
		if result[0].Type != "welcome" {
			t.Errorf("expected type 'welcome', got %s", result[0].Type)
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
			GetEmailTemplatesByCampaign(gomock.Any(), campaignID).
			Return(nil, nil)

		result, err := processor.ListEmailTemplates(ctx, accountID, campaignID, nil)

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

	t.Run("returns error for invalid template type filter", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		invalidType := "invalid_type"
		_, err := processor.ListEmailTemplates(ctx, accountID, campaignID, &invalidType)

		if !errors.Is(err, ErrInvalidTemplateType) {
			t.Errorf("expected ErrInvalidTemplateType, got %v", err)
		}
	})

	t.Run("returns error when campaign not found", func(t *testing.T) {
		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(store.Campaign{}, store.ErrNotFound)

		_, err := processor.ListEmailTemplates(ctx, accountID, campaignID, nil)

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

		_, err := processor.ListEmailTemplates(ctx, accountID, campaignID, nil)

		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})
}

func TestUpdateEmailTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailTemplateStore(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockEmailService, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	templateID := uuid.New()

	t.Run("successfully updates email template", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		existingTemplate := store.EmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       "Old Name",
			Type:       "welcome",
			Subject:    "Old Subject",
		}

		newName := "New Name"
		newSubject := "New Subject"

		updatedTemplate := store.EmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       newName,
			Type:       "welcome",
			Subject:    newSubject,
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(existingTemplate, nil)

		mockStore.EXPECT().
			UpdateEmailTemplate(gomock.Any(), templateID, store.UpdateEmailTemplateParams{
				Name:    &newName,
				Subject: &newSubject,
			}).
			Return(updatedTemplate, nil)

		req := UpdateEmailTemplateRequest{
			Name:    &newName,
			Subject: &newSubject,
		}

		result, err := processor.UpdateEmailTemplate(ctx, accountID, campaignID, templateID, req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Name != newName {
			t.Errorf("expected name %s, got %s", newName, result.Name)
		}
		if result.Subject != newSubject {
			t.Errorf("expected subject %s, got %s", newSubject, result.Subject)
		}
	})

	t.Run("returns error for invalid HTML template content", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		existingTemplate := store.EmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       "Template",
			Type:       "welcome",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(existingTemplate, nil)

		invalidHTML := "{{.Invalid"
		req := UpdateEmailTemplateRequest{
			HTMLBody: &invalidHTML,
		}

		_, err := processor.UpdateEmailTemplate(ctx, accountID, campaignID, templateID, req)

		if !errors.Is(err, ErrInvalidTemplateContent) {
			t.Errorf("expected ErrInvalidTemplateContent, got %v", err)
		}
	})

	t.Run("returns error for invalid text template content", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		existingTemplate := store.EmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       "Template",
			Type:       "welcome",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(existingTemplate, nil)

		invalidText := "{{.Invalid"
		req := UpdateEmailTemplateRequest{
			TextBody: &invalidText,
		}

		_, err := processor.UpdateEmailTemplate(ctx, accountID, campaignID, templateID, req)

		if !errors.Is(err, ErrInvalidTemplateContent) {
			t.Errorf("expected ErrInvalidTemplateContent, got %v", err)
		}
	})

	t.Run("returns error when campaign not found", func(t *testing.T) {
		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(store.Campaign{}, store.ErrNotFound)

		req := UpdateEmailTemplateRequest{}

		_, err := processor.UpdateEmailTemplate(ctx, accountID, campaignID, templateID, req)

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

		req := UpdateEmailTemplateRequest{}

		_, err := processor.UpdateEmailTemplate(ctx, accountID, campaignID, templateID, req)

		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
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
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(store.EmailTemplate{}, store.ErrNotFound)

		req := UpdateEmailTemplateRequest{}

		_, err := processor.UpdateEmailTemplate(ctx, accountID, campaignID, templateID, req)

		if !errors.Is(err, ErrTemplateNotFound) {
			t.Errorf("expected ErrTemplateNotFound, got %v", err)
		}
	})

	t.Run("returns error when template belongs to different campaign", func(t *testing.T) {
		differentCampaignID := uuid.New()
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		template := store.EmailTemplate{
			ID:         templateID,
			CampaignID: differentCampaignID,
			Name:       "Template",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(template, nil)

		req := UpdateEmailTemplateRequest{}

		_, err := processor.UpdateEmailTemplate(ctx, accountID, campaignID, templateID, req)

		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("returns error when store update fails", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		existingTemplate := store.EmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       "Template",
			Type:       "welcome",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(existingTemplate, nil)

		mockStore.EXPECT().
			UpdateEmailTemplate(gomock.Any(), templateID, gomock.Any()).
			Return(store.EmailTemplate{}, store.ErrNotFound)

		newName := "New Name"
		req := UpdateEmailTemplateRequest{
			Name: &newName,
		}

		_, err := processor.UpdateEmailTemplate(ctx, accountID, campaignID, templateID, req)

		if !errors.Is(err, ErrTemplateNotFound) {
			t.Errorf("expected ErrTemplateNotFound, got %v", err)
		}
	})
}

func TestDeleteEmailTemplate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailTemplateStore(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockEmailService, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	templateID := uuid.New()

	t.Run("successfully deletes email template", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		existingTemplate := store.EmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       "Template",
			Type:       "welcome",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(existingTemplate, nil)

		mockStore.EXPECT().
			DeleteEmailTemplate(gomock.Any(), templateID).
			Return(nil)

		err := processor.DeleteEmailTemplate(ctx, accountID, campaignID, templateID)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("returns error when campaign not found", func(t *testing.T) {
		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(store.Campaign{}, store.ErrNotFound)

		err := processor.DeleteEmailTemplate(ctx, accountID, campaignID, templateID)

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

		err := processor.DeleteEmailTemplate(ctx, accountID, campaignID, templateID)

		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
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
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(store.EmailTemplate{}, store.ErrNotFound)

		err := processor.DeleteEmailTemplate(ctx, accountID, campaignID, templateID)

		if !errors.Is(err, ErrTemplateNotFound) {
			t.Errorf("expected ErrTemplateNotFound, got %v", err)
		}
	})

	t.Run("returns error when template belongs to different campaign", func(t *testing.T) {
		differentCampaignID := uuid.New()
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		template := store.EmailTemplate{
			ID:         templateID,
			CampaignID: differentCampaignID,
			Name:       "Template",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(template, nil)

		err := processor.DeleteEmailTemplate(ctx, accountID, campaignID, templateID)

		if !errors.Is(err, ErrUnauthorized) {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("returns error when store delete fails", func(t *testing.T) {
		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		existingTemplate := store.EmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       "Template",
			Type:       "welcome",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(existingTemplate, nil)

		mockStore.EXPECT().
			DeleteEmailTemplate(gomock.Any(), templateID).
			Return(store.ErrNotFound)

		err := processor.DeleteEmailTemplate(ctx, accountID, campaignID, templateID)

		if !errors.Is(err, ErrTemplateNotFound) {
			t.Errorf("expected ErrTemplateNotFound, got %v", err)
		}
	})
}

func TestSendTestEmail(t *testing.T) {
	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	templateID := uuid.New()
	logger := observability.NewLogger()

	t.Run("successfully sends test email", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockStore := NewMockEmailTemplateStore(ctrl)
		mockEmailService := NewMockEmailService(ctrl)
		processor := New(mockStore, mockEmailService, logger)

		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		template := store.EmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       "Welcome Email",
			Type:       "welcome",
			Subject:    "Welcome!",
			HTMLBody:   "<h1>Hello {{.first_name}}</h1>",
			TextBody:   "Hello",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(template, nil)

		mockEmailService.EXPECT().
			SendEmail(gomock.Any(), "test@example.com", "Welcome!", gomock.Any()).
			Return(nil)

		req := SendTestEmailRequest{
			RecipientEmail: "test@example.com",
			TestData: map[string]interface{}{
				"first_name": "John",
			},
		}

		err := processor.SendTestEmail(ctx, accountID, campaignID, templateID, req)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("returns error when template not found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockStore := NewMockEmailTemplateStore(ctrl)
		mockEmailService := NewMockEmailService(ctrl)
		processor := New(mockStore, mockEmailService, logger)

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(store.Campaign{}, store.ErrNotFound)

		req := SendTestEmailRequest{
			RecipientEmail: "test@example.com",
		}

		err := processor.SendTestEmail(ctx, accountID, campaignID, templateID, req)

		if !errors.Is(err, ErrCampaignNotFound) {
			t.Errorf("expected ErrCampaignNotFound, got %v", err)
		}
	})

	t.Run("returns error when template rendering fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockStore := NewMockEmailTemplateStore(ctrl)
		mockEmailService := NewMockEmailService(ctrl)
		processor := New(mockStore, mockEmailService, logger)

		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		// Use an invalid template syntax that fails during parsing
		template := store.EmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       "Welcome Email",
			Type:       "welcome",
			Subject:    "Welcome!",
			HTMLBody:   "{{.invalid", // This is invalid template syntax
			TextBody:   "Hello",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(template, nil)

		req := SendTestEmailRequest{
			RecipientEmail: "test@example.com",
			TestData:       map[string]interface{}{},
		}

		err := processor.SendTestEmail(ctx, accountID, campaignID, templateID, req)

		if !errors.Is(err, ErrTestEmailFailed) {
			t.Errorf("expected ErrTestEmailFailed, got %v", err)
		}
	})

	t.Run("returns error when email service fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockStore := NewMockEmailTemplateStore(ctrl)
		mockEmailService := NewMockEmailService(ctrl)
		processor := New(mockStore, mockEmailService, logger)

		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
			Status:    "active",
		}

		template := store.EmailTemplate{
			ID:         templateID,
			CampaignID: campaignID,
			Name:       "Welcome Email",
			Type:       "welcome",
			Subject:    "Welcome!",
			HTMLBody:   "<h1>Hello</h1>",
			TextBody:   "Hello",
		}

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetEmailTemplateByID(gomock.Any(), templateID).
			Return(template, nil)

		mockEmailService.EXPECT().
			SendEmail(gomock.Any(), "test@example.com", "Welcome!", gomock.Any()).
			Return(errors.New("email service error"))

		req := SendTestEmailRequest{
			RecipientEmail: "test@example.com",
		}

		err := processor.SendTestEmail(ctx, accountID, campaignID, templateID, req)

		if !errors.Is(err, ErrTestEmailFailed) {
			t.Errorf("expected ErrTestEmailFailed, got %v", err)
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
