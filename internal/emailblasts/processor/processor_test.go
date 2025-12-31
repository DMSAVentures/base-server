package processor

import (
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateEmailBlast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()
	campaignID := uuid.New()
	segmentID := uuid.New()
	templateID := uuid.New()
	blastID := uuid.New()

	t.Run("successfully creates email blast", func(t *testing.T) {
		segment := store.Segment{
			ID:         segmentID,
			CampaignID: campaignID,
			Name:       "Test Segment",
		}

		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
		}

		template := store.BlastEmailTemplate{
			ID:        templateID,
			AccountID: accountID,
			Name:      "Test Template",
		}

		expectedBlast := store.EmailBlast{
			ID:              blastID,
			AccountID:       accountID,
			BlastTemplateID: templateID,
			Name:            "Test Blast",
			Subject:         "Test Subject",
			Status:          string(store.EmailBlastStatusDraft),
		}

		mockTierChecker.EXPECT().
			HasFeatureByAccountID(gomock.Any(), accountID, "email_blasts").
			Return(true, nil)

		mockStore.EXPECT().
			GetSegmentByID(gomock.Any(), segmentID).
			Return(segment, nil)

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetBlastEmailTemplateByID(gomock.Any(), templateID).
			Return(template, nil)

		mockStore.EXPECT().
			CreateEmailBlast(gomock.Any(), gomock.Any()).
			Return(expectedBlast, nil)

		req := CreateEmailBlastRequest{
			Name:            "Test Blast",
			SegmentIDs:      []uuid.UUID{segmentID},
			BlastTemplateID: templateID,
			Subject:         "Test Subject",
			BatchSize:       100,
		}

		result, err := processor.CreateEmailBlast(ctx, accountID, nil, req)

		require.NoError(t, err)
		assert.Equal(t, blastID, result.ID)
		assert.Equal(t, "Test Blast", result.Name)
	})

	t.Run("returns error when email blasts feature not available", func(t *testing.T) {
		mockTierChecker.EXPECT().
			HasFeatureByAccountID(gomock.Any(), accountID, "email_blasts").
			Return(false, nil)

		req := CreateEmailBlastRequest{
			Name:            "Test Blast",
			SegmentIDs:      []uuid.UUID{segmentID},
			BlastTemplateID: templateID,
			Subject:         "Test Subject",
		}

		_, err := processor.CreateEmailBlast(ctx, accountID, nil, req)

		assert.ErrorIs(t, err, ErrEmailBlastsNotAvailable)
	})

	t.Run("returns error when no segments provided", func(t *testing.T) {
		mockTierChecker.EXPECT().
			HasFeatureByAccountID(gomock.Any(), accountID, "email_blasts").
			Return(true, nil)

		req := CreateEmailBlastRequest{
			Name:            "Test Blast",
			SegmentIDs:      []uuid.UUID{},
			BlastTemplateID: templateID,
			Subject:         "Test Subject",
		}

		_, err := processor.CreateEmailBlast(ctx, accountID, nil, req)

		assert.ErrorIs(t, err, ErrSegmentNotFound)
	})

	t.Run("returns error when segment not found", func(t *testing.T) {
		mockTierChecker.EXPECT().
			HasFeatureByAccountID(gomock.Any(), accountID, "email_blasts").
			Return(true, nil)

		mockStore.EXPECT().
			GetSegmentByID(gomock.Any(), segmentID).
			Return(store.Segment{}, store.ErrNotFound)

		req := CreateEmailBlastRequest{
			Name:            "Test Blast",
			SegmentIDs:      []uuid.UUID{segmentID},
			BlastTemplateID: templateID,
			Subject:         "Test Subject",
		}

		_, err := processor.CreateEmailBlast(ctx, accountID, nil, req)

		assert.ErrorIs(t, err, ErrSegmentNotFound)
	})

	t.Run("returns error when segment belongs to different account", func(t *testing.T) {
		differentAccountID := uuid.New()

		segment := store.Segment{
			ID:         segmentID,
			CampaignID: campaignID,
			Name:       "Test Segment",
		}

		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: differentAccountID, // Different account
			Name:      "Test Campaign",
		}

		mockTierChecker.EXPECT().
			HasFeatureByAccountID(gomock.Any(), accountID, "email_blasts").
			Return(true, nil)

		mockStore.EXPECT().
			GetSegmentByID(gomock.Any(), segmentID).
			Return(segment, nil)

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		req := CreateEmailBlastRequest{
			Name:            "Test Blast",
			SegmentIDs:      []uuid.UUID{segmentID},
			BlastTemplateID: templateID,
			Subject:         "Test Subject",
		}

		_, err := processor.CreateEmailBlast(ctx, accountID, nil, req)

		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("returns error when template not found", func(t *testing.T) {
		segment := store.Segment{
			ID:         segmentID,
			CampaignID: campaignID,
			Name:       "Test Segment",
		}

		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
		}

		mockTierChecker.EXPECT().
			HasFeatureByAccountID(gomock.Any(), accountID, "email_blasts").
			Return(true, nil)

		mockStore.EXPECT().
			GetSegmentByID(gomock.Any(), segmentID).
			Return(segment, nil)

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetBlastEmailTemplateByID(gomock.Any(), templateID).
			Return(store.BlastEmailTemplate{}, store.ErrNotFound)

		req := CreateEmailBlastRequest{
			Name:            "Test Blast",
			SegmentIDs:      []uuid.UUID{segmentID},
			BlastTemplateID: templateID,
			Subject:         "Test Subject",
		}

		_, err := processor.CreateEmailBlast(ctx, accountID, nil, req)

		assert.ErrorIs(t, err, ErrTemplateNotFound)
	})

	t.Run("returns error when scheduled time is in the past", func(t *testing.T) {
		segment := store.Segment{
			ID:         segmentID,
			CampaignID: campaignID,
			Name:       "Test Segment",
		}

		campaign := store.Campaign{
			ID:        campaignID,
			AccountID: accountID,
			Name:      "Test Campaign",
		}

		template := store.BlastEmailTemplate{
			ID:        templateID,
			AccountID: accountID,
			Name:      "Test Template",
		}

		pastTime := time.Now().Add(-1 * time.Hour)

		mockTierChecker.EXPECT().
			HasFeatureByAccountID(gomock.Any(), accountID, "email_blasts").
			Return(true, nil)

		mockStore.EXPECT().
			GetSegmentByID(gomock.Any(), segmentID).
			Return(segment, nil)

		mockStore.EXPECT().
			GetCampaignByID(gomock.Any(), campaignID).
			Return(campaign, nil)

		mockStore.EXPECT().
			GetBlastEmailTemplateByID(gomock.Any(), templateID).
			Return(template, nil)

		req := CreateEmailBlastRequest{
			Name:            "Test Blast",
			SegmentIDs:      []uuid.UUID{segmentID},
			BlastTemplateID: templateID,
			Subject:         "Test Subject",
			ScheduledAt:     &pastTime,
		}

		_, err := processor.CreateEmailBlast(ctx, accountID, nil, req)

		assert.ErrorIs(t, err, ErrInvalidScheduleTime)
	})
}

func TestGetEmailBlast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()
	blastID := uuid.New()

	t.Run("successfully gets email blast", func(t *testing.T) {
		expectedBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusDraft),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(expectedBlast, nil)

		result, err := processor.GetEmailBlast(ctx, accountID, blastID)

		require.NoError(t, err)
		assert.Equal(t, blastID, result.ID)
		assert.Equal(t, "Test Blast", result.Name)
	})

	t.Run("returns error when blast not found", func(t *testing.T) {
		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(store.EmailBlast{}, store.ErrNotFound)

		_, err := processor.GetEmailBlast(ctx, accountID, blastID)

		assert.ErrorIs(t, err, ErrBlastNotFound)
	})

	t.Run("returns error when blast belongs to different account", func(t *testing.T) {
		differentAccountID := uuid.New()

		blast := store.EmailBlast{
			ID:        blastID,
			AccountID: differentAccountID,
			Name:      "Test Blast",
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(blast, nil)

		_, err := processor.GetEmailBlast(ctx, accountID, blastID)

		assert.ErrorIs(t, err, ErrUnauthorized)
	})
}

func TestListEmailBlasts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()

	t.Run("successfully lists email blasts", func(t *testing.T) {
		blasts := []store.EmailBlast{
			{ID: uuid.New(), AccountID: accountID, Name: "Blast 1"},
			{ID: uuid.New(), AccountID: accountID, Name: "Blast 2"},
		}

		mockStore.EXPECT().
			GetEmailBlastsByAccount(gomock.Any(), accountID, 25, 0).
			Return(blasts, nil)

		mockStore.EXPECT().
			CountEmailBlastsByAccount(gomock.Any(), accountID).
			Return(2, nil)

		req := ListEmailBlastsRequest{Page: 1, Limit: 25}
		result, err := processor.ListEmailBlasts(ctx, accountID, req)

		require.NoError(t, err)
		assert.Len(t, result.Blasts, 2)
		assert.Equal(t, 2, result.Total)
		assert.Equal(t, 1, result.Page)
		assert.Equal(t, 1, result.TotalPages)
	})

	t.Run("returns empty list when no blasts exist", func(t *testing.T) {
		mockStore.EXPECT().
			GetEmailBlastsByAccount(gomock.Any(), accountID, 25, 0).
			Return(nil, nil)

		mockStore.EXPECT().
			CountEmailBlastsByAccount(gomock.Any(), accountID).
			Return(0, nil)

		req := ListEmailBlastsRequest{Page: 1, Limit: 25}
		result, err := processor.ListEmailBlasts(ctx, accountID, req)

		require.NoError(t, err)
		assert.Empty(t, result.Blasts)
		assert.Equal(t, 0, result.Total)
	})

	t.Run("applies pagination correctly", func(t *testing.T) {
		mockStore.EXPECT().
			GetEmailBlastsByAccount(gomock.Any(), accountID, 10, 10).
			Return([]store.EmailBlast{}, nil)

		mockStore.EXPECT().
			CountEmailBlastsByAccount(gomock.Any(), accountID).
			Return(25, nil)

		req := ListEmailBlastsRequest{Page: 2, Limit: 10}
		result, err := processor.ListEmailBlasts(ctx, accountID, req)

		require.NoError(t, err)
		assert.Equal(t, 2, result.Page)
		assert.Equal(t, 10, result.Limit)
		assert.Equal(t, 3, result.TotalPages)
	})
}

func TestUpdateEmailBlast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()
	blastID := uuid.New()

	t.Run("successfully updates email blast", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Old Name",
			Status:    string(store.EmailBlastStatusDraft),
		}

		newName := "New Name"
		updatedBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      newName,
			Status:    string(store.EmailBlastStatusDraft),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		mockStore.EXPECT().
			UpdateEmailBlast(gomock.Any(), blastID, gomock.Any()).
			Return(updatedBlast, nil)

		req := UpdateEmailBlastRequest{Name: &newName}
		result, err := processor.UpdateEmailBlast(ctx, accountID, blastID, req)

		require.NoError(t, err)
		assert.Equal(t, newName, result.Name)
	})

	t.Run("returns error when blast not found", func(t *testing.T) {
		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(store.EmailBlast{}, store.ErrNotFound)

		newName := "New Name"
		req := UpdateEmailBlastRequest{Name: &newName}
		_, err := processor.UpdateEmailBlast(ctx, accountID, blastID, req)

		assert.ErrorIs(t, err, ErrBlastNotFound)
	})

	t.Run("returns error when blast is not in draft status", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusSending),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		newName := "New Name"
		req := UpdateEmailBlastRequest{Name: &newName}
		_, err := processor.UpdateEmailBlast(ctx, accountID, blastID, req)

		assert.ErrorIs(t, err, ErrBlastCannotModify)
	})
}

func TestDeleteEmailBlast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()
	blastID := uuid.New()

	t.Run("successfully deletes email blast", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusDraft),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		mockStore.EXPECT().
			DeleteEmailBlast(gomock.Any(), blastID).
			Return(nil)

		err := processor.DeleteEmailBlast(ctx, accountID, blastID)

		require.NoError(t, err)
	})

	t.Run("returns error when blast not found", func(t *testing.T) {
		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(store.EmailBlast{}, store.ErrNotFound)

		err := processor.DeleteEmailBlast(ctx, accountID, blastID)

		assert.ErrorIs(t, err, ErrBlastNotFound)
	})

	t.Run("returns error when blast belongs to different account", func(t *testing.T) {
		differentAccountID := uuid.New()

		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: differentAccountID,
			Name:      "Test Blast",
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		err := processor.DeleteEmailBlast(ctx, accountID, blastID)

		assert.ErrorIs(t, err, ErrUnauthorized)
	})
}

func TestScheduleBlast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()
	blastID := uuid.New()

	t.Run("successfully schedules blast", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusDraft),
		}

		futureTime := time.Now().Add(24 * time.Hour)
		scheduledBlast := store.EmailBlast{
			ID:          blastID,
			AccountID:   accountID,
			Name:        "Test Blast",
			Status:      string(store.EmailBlastStatusScheduled),
			ScheduledAt: &futureTime,
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		mockStore.EXPECT().
			ScheduleBlast(gomock.Any(), blastID, gomock.Any()).
			Return(scheduledBlast, nil)

		result, err := processor.ScheduleBlast(ctx, accountID, blastID, futureTime)

		require.NoError(t, err)
		assert.Equal(t, string(store.EmailBlastStatusScheduled), result.Status)
	})

	t.Run("returns error when scheduled time is in the past", func(t *testing.T) {
		pastTime := time.Now().Add(-1 * time.Hour)

		_, err := processor.ScheduleBlast(ctx, accountID, blastID, pastTime)

		assert.ErrorIs(t, err, ErrInvalidScheduleTime)
	})

	t.Run("returns error when blast is not in draft status", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusSending),
		}

		futureTime := time.Now().Add(24 * time.Hour)

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		_, err := processor.ScheduleBlast(ctx, accountID, blastID, futureTime)

		assert.ErrorIs(t, err, ErrBlastCannotModify)
	})
}

func TestSendBlastNow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()
	blastID := uuid.New()

	t.Run("successfully sends blast now", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusDraft),
		}

		processingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusProcessing),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		mockStore.EXPECT().
			UpdateEmailBlastStatus(gomock.Any(), blastID, string(store.EmailBlastStatusProcessing), nil).
			Return(processingBlast, nil)

		mockEventDispatcher.EXPECT().
			DispatchBlastStarted(gomock.Any(), accountID, blastID).
			Return(nil)

		result, err := processor.SendBlastNow(ctx, accountID, blastID)

		require.NoError(t, err)
		assert.Equal(t, string(store.EmailBlastStatusProcessing), result.Status)
	})

	t.Run("returns error when blast cannot be started", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusCompleted),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		_, err := processor.SendBlastNow(ctx, accountID, blastID)

		assert.ErrorIs(t, err, ErrBlastCannotStart)
	})

	t.Run("reverts status when event dispatch fails", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusDraft),
		}

		processingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusProcessing),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		mockStore.EXPECT().
			UpdateEmailBlastStatus(gomock.Any(), blastID, string(store.EmailBlastStatusProcessing), nil).
			Return(processingBlast, nil)

		mockEventDispatcher.EXPECT().
			DispatchBlastStarted(gomock.Any(), accountID, blastID).
			Return(errors.New("dispatch failed"))

		// Expect revert to draft
		mockStore.EXPECT().
			UpdateEmailBlastStatus(gomock.Any(), blastID, string(store.EmailBlastStatusDraft), nil).
			Return(existingBlast, nil)

		_, err := processor.SendBlastNow(ctx, accountID, blastID)

		assert.Error(t, err)
	})
}

func TestPauseBlast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()
	blastID := uuid.New()

	t.Run("successfully pauses sending blast", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusSending),
		}

		pausedBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusPaused),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		mockStore.EXPECT().
			UpdateEmailBlastStatus(gomock.Any(), blastID, string(store.EmailBlastStatusPaused), nil).
			Return(pausedBlast, nil)

		result, err := processor.PauseBlast(ctx, accountID, blastID)

		require.NoError(t, err)
		assert.Equal(t, string(store.EmailBlastStatusPaused), result.Status)
	})

	t.Run("returns error when blast cannot be paused", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusDraft),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		_, err := processor.PauseBlast(ctx, accountID, blastID)

		assert.ErrorIs(t, err, ErrBlastCannotPause)
	})
}

func TestResumeBlast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()
	blastID := uuid.New()

	t.Run("successfully resumes paused blast", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:           blastID,
			AccountID:    accountID,
			Name:         "Test Blast",
			Status:       string(store.EmailBlastStatusPaused),
			CurrentBatch: 5,
		}

		resumedBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusSending),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		mockStore.EXPECT().
			UpdateEmailBlastStatus(gomock.Any(), blastID, string(store.EmailBlastStatusSending), nil).
			Return(resumedBlast, nil)

		mockEventDispatcher.EXPECT().
			DispatchBlastBatchSend(gomock.Any(), accountID, blastID, 6). // CurrentBatch + 1
			Return(nil)

		result, err := processor.ResumeBlast(ctx, accountID, blastID)

		require.NoError(t, err)
		assert.Equal(t, string(store.EmailBlastStatusSending), result.Status)
	})

	t.Run("returns error when blast is not paused", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusDraft),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		_, err := processor.ResumeBlast(ctx, accountID, blastID)

		assert.ErrorIs(t, err, ErrBlastCannotResume)
	})
}

func TestCancelBlast(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()
	blastID := uuid.New()

	validStatuses := []string{
		string(store.EmailBlastStatusDraft),
		string(store.EmailBlastStatusScheduled),
		string(store.EmailBlastStatusProcessing),
		string(store.EmailBlastStatusSending),
		string(store.EmailBlastStatusPaused),
	}

	for _, status := range validStatuses {
		t.Run("successfully cancels blast from "+status+" status", func(t *testing.T) {
			existingBlast := store.EmailBlast{
				ID:        blastID,
				AccountID: accountID,
				Name:      "Test Blast",
				Status:    status,
			}

			cancelledBlast := store.EmailBlast{
				ID:        blastID,
				AccountID: accountID,
				Name:      "Test Blast",
				Status:    string(store.EmailBlastStatusCancelled),
			}

			mockStore.EXPECT().
				GetEmailBlastByID(gomock.Any(), blastID).
				Return(existingBlast, nil)

			mockStore.EXPECT().
				UpdateEmailBlastStatus(gomock.Any(), blastID, string(store.EmailBlastStatusCancelled), nil).
				Return(cancelledBlast, nil)

			result, err := processor.CancelBlast(ctx, accountID, blastID)

			require.NoError(t, err)
			assert.Equal(t, string(store.EmailBlastStatusCancelled), result.Status)
		})
	}

	t.Run("returns error when blast is already completed", func(t *testing.T) {
		existingBlast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
			Status:    string(store.EmailBlastStatusCompleted),
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(existingBlast, nil)

		_, err := processor.CancelBlast(ctx, accountID, blastID)

		assert.ErrorIs(t, err, ErrBlastCannotCancel)
	})
}

func TestGetBlastAnalytics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()
	blastID := uuid.New()

	t.Run("successfully gets blast analytics", func(t *testing.T) {
		blast := store.EmailBlast{
			ID:              blastID,
			AccountID:       accountID,
			Name:            "Test Blast",
			Status:          string(store.EmailBlastStatusCompleted),
			TotalRecipients: 100,
		}

		stats := store.BlastRecipientStats{
			Pending:   0,
			Sent:      80,
			Delivered: 10,
			Opened:    5,
			Clicked:   3,
			Bounced:   2,
			Failed:    5,
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(blast, nil)

		mockStore.EXPECT().
			GetBlastRecipientStats(gomock.Any(), blastID).
			Return(stats, nil)

		result, err := processor.GetBlastAnalytics(ctx, accountID, blastID)

		require.NoError(t, err)
		assert.Equal(t, blastID, result.BlastID)
		assert.Equal(t, 100, result.TotalRecipients)
		assert.Equal(t, 98, result.Sent) // Sent + Delivered + Opened + Clicked
		assert.Equal(t, 2, result.Bounced)
		assert.Equal(t, 5, result.Failed)
	})

	t.Run("returns error when blast not found", func(t *testing.T) {
		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(store.EmailBlast{}, store.ErrNotFound)

		_, err := processor.GetBlastAnalytics(ctx, accountID, blastID)

		assert.ErrorIs(t, err, ErrBlastNotFound)
	})
}

func TestListBlastRecipients(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockEmailBlastStore(ctrl)
	mockTierChecker := NewMockTierChecker(ctrl)
	mockEventDispatcher := NewMockEventDispatcher(ctrl)
	logger := observability.NewLogger()
	processor := New(mockStore, mockTierChecker, mockEventDispatcher, logger)

	ctx := context.Background()
	accountID := uuid.New()
	blastID := uuid.New()

	t.Run("successfully lists blast recipients", func(t *testing.T) {
		blast := store.EmailBlast{
			ID:        blastID,
			AccountID: accountID,
			Name:      "Test Blast",
		}

		recipients := []store.BlastRecipient{
			{ID: uuid.New(), BlastID: blastID, Email: "user1@example.com"},
			{ID: uuid.New(), BlastID: blastID, Email: "user2@example.com"},
		}

		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(blast, nil)

		mockStore.EXPECT().
			GetBlastRecipientsByBlast(gomock.Any(), blastID, 25, 0).
			Return(recipients, nil)

		mockStore.EXPECT().
			CountBlastRecipientsByBlast(gomock.Any(), blastID).
			Return(2, nil)

		req := ListBlastRecipientsRequest{Page: 1, Limit: 25}
		result, err := processor.ListBlastRecipients(ctx, accountID, blastID, req)

		require.NoError(t, err)
		assert.Len(t, result.Recipients, 2)
		assert.Equal(t, 2, result.Total)
	})

	t.Run("returns error when blast not found", func(t *testing.T) {
		mockStore.EXPECT().
			GetEmailBlastByID(gomock.Any(), blastID).
			Return(store.EmailBlast{}, store.ErrNotFound)

		req := ListBlastRecipientsRequest{Page: 1, Limit: 25}
		_, err := processor.ListBlastRecipients(ctx, accountID, blastID, req)

		assert.ErrorIs(t, err, ErrBlastNotFound)
	})
}
