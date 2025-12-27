package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateEmailBlastParams represents parameters for creating an email blast
type CreateEmailBlastParams struct {
	CampaignID            uuid.UUID
	SegmentID             uuid.UUID
	TemplateID            uuid.UUID
	Name                  string
	Subject               string
	ScheduledAt           *time.Time
	BatchSize             int
	SendThrottlePerSecond *int
	CreatedBy             *uuid.UUID
}

const sqlCreateEmailBlast = `
INSERT INTO email_blasts (campaign_id, segment_id, template_id, name, subject, scheduled_at, batch_size, send_throttle_per_second, created_by, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, CASE WHEN $6 IS NOT NULL THEN 'scheduled' ELSE 'draft' END)
RETURNING id, campaign_id, segment_id, template_id, name, subject, scheduled_at, started_at, completed_at, status, total_recipients, sent_count, delivered_count, opened_count, clicked_count, bounced_count, failed_count, batch_size, current_batch, last_batch_at, error_message, send_throttle_per_second, created_by, created_at, updated_at, deleted_at
`

// CreateEmailBlast creates a new email blast
func (s *Store) CreateEmailBlast(ctx context.Context, params CreateEmailBlastParams) (EmailBlast, error) {
	var blast EmailBlast
	err := s.db.GetContext(ctx, &blast, sqlCreateEmailBlast,
		params.CampaignID,
		params.SegmentID,
		params.TemplateID,
		params.Name,
		params.Subject,
		params.ScheduledAt,
		params.BatchSize,
		params.SendThrottlePerSecond,
		params.CreatedBy)
	if err != nil {
		return EmailBlast{}, fmt.Errorf("failed to create email blast: %w", err)
	}
	return blast, nil
}

const sqlGetEmailBlastByID = `
SELECT id, campaign_id, segment_id, template_id, name, subject, scheduled_at, started_at, completed_at, status, total_recipients, sent_count, delivered_count, opened_count, clicked_count, bounced_count, failed_count, batch_size, current_batch, last_batch_at, error_message, send_throttle_per_second, created_by, created_at, updated_at, deleted_at
FROM email_blasts
WHERE id = $1 AND deleted_at IS NULL
`

// GetEmailBlastByID retrieves an email blast by ID
func (s *Store) GetEmailBlastByID(ctx context.Context, blastID uuid.UUID) (EmailBlast, error) {
	var blast EmailBlast
	err := s.db.GetContext(ctx, &blast, sqlGetEmailBlastByID, blastID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailBlast{}, ErrNotFound
		}
		return EmailBlast{}, fmt.Errorf("failed to get email blast: %w", err)
	}
	return blast, nil
}

const sqlGetEmailBlastsByCampaign = `
SELECT id, campaign_id, segment_id, template_id, name, subject, scheduled_at, started_at, completed_at, status, total_recipients, sent_count, delivered_count, opened_count, clicked_count, bounced_count, failed_count, batch_size, current_batch, last_batch_at, error_message, send_throttle_per_second, created_by, created_at, updated_at, deleted_at
FROM email_blasts
WHERE campaign_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

// GetEmailBlastsByCampaign retrieves email blasts for a campaign with pagination
func (s *Store) GetEmailBlastsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]EmailBlast, error) {
	var blasts []EmailBlast
	err := s.db.SelectContext(ctx, &blasts, sqlGetEmailBlastsByCampaign, campaignID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get email blasts: %w", err)
	}
	return blasts, nil
}

const sqlCountEmailBlastsByCampaign = `
SELECT COUNT(*)
FROM email_blasts
WHERE campaign_id = $1 AND deleted_at IS NULL
`

// CountEmailBlastsByCampaign counts email blasts for a campaign
func (s *Store) CountEmailBlastsByCampaign(ctx context.Context, campaignID uuid.UUID) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlCountEmailBlastsByCampaign, campaignID)
	if err != nil {
		return 0, fmt.Errorf("failed to count email blasts: %w", err)
	}
	return count, nil
}

// UpdateEmailBlastParams represents parameters for updating an email blast
type UpdateEmailBlastParams struct {
	Name        *string
	Subject     *string
	ScheduledAt *time.Time
	BatchSize   *int
}

const sqlUpdateEmailBlast = `
UPDATE email_blasts
SET name = COALESCE($2, name),
    subject = COALESCE($3, subject),
    scheduled_at = COALESCE($4, scheduled_at),
    batch_size = COALESCE($5, batch_size),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL AND status = 'draft'
RETURNING id, campaign_id, segment_id, template_id, name, subject, scheduled_at, started_at, completed_at, status, total_recipients, sent_count, delivered_count, opened_count, clicked_count, bounced_count, failed_count, batch_size, current_batch, last_batch_at, error_message, send_throttle_per_second, created_by, created_at, updated_at, deleted_at
`

// UpdateEmailBlast updates an email blast (only if in draft status)
func (s *Store) UpdateEmailBlast(ctx context.Context, blastID uuid.UUID, params UpdateEmailBlastParams) (EmailBlast, error) {
	var blast EmailBlast
	err := s.db.GetContext(ctx, &blast, sqlUpdateEmailBlast,
		blastID,
		params.Name,
		params.Subject,
		params.ScheduledAt,
		params.BatchSize)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailBlast{}, ErrNotFound
		}
		return EmailBlast{}, fmt.Errorf("failed to update email blast: %w", err)
	}
	return blast, nil
}

const sqlDeleteEmailBlast = `
UPDATE email_blasts
SET deleted_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL AND status IN ('draft', 'scheduled', 'cancelled', 'completed', 'failed')
`

// DeleteEmailBlast soft deletes an email blast (only if not actively sending)
func (s *Store) DeleteEmailBlast(ctx context.Context, blastID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlDeleteEmailBlast, blastID)
	if err != nil {
		return fmt.Errorf("failed to delete email blast: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlUpdateEmailBlastStatus = `
UPDATE email_blasts
SET status = $2,
    error_message = $3,
    started_at = CASE WHEN $2 IN ('processing', 'sending') AND started_at IS NULL THEN CURRENT_TIMESTAMP ELSE started_at END,
    completed_at = CASE WHEN $2 IN ('completed', 'cancelled', 'failed') THEN CURRENT_TIMESTAMP ELSE completed_at END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, campaign_id, segment_id, template_id, name, subject, scheduled_at, started_at, completed_at, status, total_recipients, sent_count, delivered_count, opened_count, clicked_count, bounced_count, failed_count, batch_size, current_batch, last_batch_at, error_message, send_throttle_per_second, created_by, created_at, updated_at, deleted_at
`

// UpdateEmailBlastStatus updates the status of an email blast
func (s *Store) UpdateEmailBlastStatus(ctx context.Context, blastID uuid.UUID, status string, errorMessage *string) (EmailBlast, error) {
	var blast EmailBlast
	err := s.db.GetContext(ctx, &blast, sqlUpdateEmailBlastStatus, blastID, status, errorMessage)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailBlast{}, ErrNotFound
		}
		return EmailBlast{}, fmt.Errorf("failed to update email blast status: %w", err)
	}
	return blast, nil
}

const sqlUpdateEmailBlastTotalRecipients = `
UPDATE email_blasts
SET total_recipients = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// UpdateEmailBlastTotalRecipients updates the total recipient count for a blast
func (s *Store) UpdateEmailBlastTotalRecipients(ctx context.Context, blastID uuid.UUID, totalRecipients int) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateEmailBlastTotalRecipients, blastID, totalRecipients)
	if err != nil {
		return fmt.Errorf("failed to update email blast total recipients: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlIncrementEmailBlastSentCount = `
UPDATE email_blasts
SET sent_count = sent_count + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// IncrementEmailBlastSentCount increments the sent count for a blast
func (s *Store) IncrementEmailBlastSentCount(ctx context.Context, blastID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementEmailBlastSentCount, blastID)
	if err != nil {
		return fmt.Errorf("failed to increment email blast sent count: %w", err)
	}
	return nil
}

const sqlIncrementEmailBlastFailedCount = `
UPDATE email_blasts
SET failed_count = failed_count + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// IncrementEmailBlastFailedCount increments the failed count for a blast
func (s *Store) IncrementEmailBlastFailedCount(ctx context.Context, blastID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlIncrementEmailBlastFailedCount, blastID)
	if err != nil {
		return fmt.Errorf("failed to increment email blast failed count: %w", err)
	}
	return nil
}

const sqlUpdateEmailBlastProgress = `
UPDATE email_blasts
SET current_batch = $2,
    last_batch_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// UpdateEmailBlastProgress updates the batch progress for a blast
func (s *Store) UpdateEmailBlastProgress(ctx context.Context, blastID uuid.UUID, currentBatch int) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateEmailBlastProgress, blastID, currentBatch)
	if err != nil {
		return fmt.Errorf("failed to update email blast progress: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlGetScheduledBlasts = `
SELECT id, campaign_id, segment_id, template_id, name, subject, scheduled_at, started_at, completed_at, status, total_recipients, sent_count, delivered_count, opened_count, clicked_count, bounced_count, failed_count, batch_size, current_batch, last_batch_at, error_message, send_throttle_per_second, created_by, created_at, updated_at, deleted_at
FROM email_blasts
WHERE status = 'scheduled' AND scheduled_at <= $1 AND deleted_at IS NULL
ORDER BY scheduled_at ASC
`

// GetScheduledBlasts retrieves blasts that are scheduled and ready to send
func (s *Store) GetScheduledBlasts(ctx context.Context, beforeTime time.Time) ([]EmailBlast, error) {
	var blasts []EmailBlast
	err := s.db.SelectContext(ctx, &blasts, sqlGetScheduledBlasts, beforeTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled blasts: %w", err)
	}
	return blasts, nil
}

// ============================================================================
// Blast Recipients
// ============================================================================

// CreateBlastRecipientParams represents parameters for creating a blast recipient
type CreateBlastRecipientParams struct {
	BlastID     uuid.UUID
	UserID      uuid.UUID
	Email       string
	BatchNumber *int
}

const sqlCreateBlastRecipient = `
INSERT INTO blast_recipients (blast_id, user_id, email, batch_number)
VALUES ($1, $2, $3, $4)
RETURNING id, blast_id, user_id, email, status, email_log_id, queued_at, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, batch_number, created_at, updated_at
`

// CreateBlastRecipient creates a new blast recipient
func (s *Store) CreateBlastRecipient(ctx context.Context, params CreateBlastRecipientParams) (BlastRecipient, error) {
	var recipient BlastRecipient
	err := s.db.GetContext(ctx, &recipient, sqlCreateBlastRecipient,
		params.BlastID,
		params.UserID,
		params.Email,
		params.BatchNumber)
	if err != nil {
		return BlastRecipient{}, fmt.Errorf("failed to create blast recipient: %w", err)
	}
	return recipient, nil
}

const sqlCreateBlastRecipientsBulk = `
INSERT INTO blast_recipients (blast_id, user_id, email, batch_number)
VALUES ($1, $2, $3, $4)
ON CONFLICT (blast_id, user_id) DO NOTHING
`

// CreateBlastRecipientsBulk creates multiple blast recipients efficiently
func (s *Store) CreateBlastRecipientsBulk(ctx context.Context, blastID uuid.UUID, users []WaitlistUser, batchSize int) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, sqlCreateBlastRecipientsBulk)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for i, user := range users {
		batchNumber := i / batchSize
		_, err := stmt.ExecContext(ctx, blastID, user.ID, user.Email, batchNumber)
		if err != nil {
			return fmt.Errorf("failed to insert blast recipient: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

const sqlGetBlastRecipientByID = `
SELECT id, blast_id, user_id, email, status, email_log_id, queued_at, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, batch_number, created_at, updated_at
FROM blast_recipients
WHERE id = $1
`

// GetBlastRecipientByID retrieves a blast recipient by ID
func (s *Store) GetBlastRecipientByID(ctx context.Context, recipientID uuid.UUID) (BlastRecipient, error) {
	var recipient BlastRecipient
	err := s.db.GetContext(ctx, &recipient, sqlGetBlastRecipientByID, recipientID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BlastRecipient{}, ErrNotFound
		}
		return BlastRecipient{}, fmt.Errorf("failed to get blast recipient: %w", err)
	}
	return recipient, nil
}

const sqlGetPendingBlastRecipients = `
SELECT id, blast_id, user_id, email, status, email_log_id, queued_at, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, batch_number, created_at, updated_at
FROM blast_recipients
WHERE blast_id = $1 AND batch_number = $2 AND status = 'pending'
ORDER BY created_at ASC
LIMIT $3
`

// GetPendingBlastRecipients retrieves pending recipients for a specific batch
func (s *Store) GetPendingBlastRecipients(ctx context.Context, blastID uuid.UUID, batchNumber int, limit int) ([]BlastRecipient, error) {
	var recipients []BlastRecipient
	err := s.db.SelectContext(ctx, &recipients, sqlGetPendingBlastRecipients, blastID, batchNumber, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending blast recipients: %w", err)
	}
	return recipients, nil
}

const sqlGetBlastRecipientsByBlast = `
SELECT id, blast_id, user_id, email, status, email_log_id, queued_at, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, batch_number, created_at, updated_at
FROM blast_recipients
WHERE blast_id = $1
ORDER BY created_at ASC
LIMIT $2 OFFSET $3
`

// GetBlastRecipientsByBlast retrieves recipients for a blast with pagination
func (s *Store) GetBlastRecipientsByBlast(ctx context.Context, blastID uuid.UUID, limit, offset int) ([]BlastRecipient, error) {
	var recipients []BlastRecipient
	err := s.db.SelectContext(ctx, &recipients, sqlGetBlastRecipientsByBlast, blastID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get blast recipients: %w", err)
	}
	return recipients, nil
}

const sqlCountBlastRecipientsByBlast = `
SELECT COUNT(*)
FROM blast_recipients
WHERE blast_id = $1
`

// CountBlastRecipientsByBlast counts recipients for a blast
func (s *Store) CountBlastRecipientsByBlast(ctx context.Context, blastID uuid.UUID) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlCountBlastRecipientsByBlast, blastID)
	if err != nil {
		return 0, fmt.Errorf("failed to count blast recipients: %w", err)
	}
	return count, nil
}

const sqlUpdateBlastRecipientStatus = `
UPDATE blast_recipients
SET status = $2,
    email_log_id = COALESCE($3, email_log_id),
    queued_at = CASE WHEN $2 = 'queued' THEN CURRENT_TIMESTAMP ELSE queued_at END,
    sent_at = CASE WHEN $2 = 'sent' THEN CURRENT_TIMESTAMP ELSE sent_at END,
    delivered_at = CASE WHEN $2 = 'delivered' THEN CURRENT_TIMESTAMP ELSE delivered_at END,
    opened_at = CASE WHEN $2 = 'opened' THEN COALESCE(opened_at, CURRENT_TIMESTAMP) ELSE opened_at END,
    clicked_at = CASE WHEN $2 = 'clicked' THEN COALESCE(clicked_at, CURRENT_TIMESTAMP) ELSE clicked_at END,
    bounced_at = CASE WHEN $2 = 'bounced' THEN CURRENT_TIMESTAMP ELSE bounced_at END,
    failed_at = CASE WHEN $2 = 'failed' THEN CURRENT_TIMESTAMP ELSE failed_at END,
    error_message = COALESCE($4, error_message),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
`

// UpdateBlastRecipientStatus updates the status of a blast recipient
func (s *Store) UpdateBlastRecipientStatus(ctx context.Context, recipientID uuid.UUID, status string, emailLogID *uuid.UUID, errorMessage *string) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateBlastRecipientStatus, recipientID, status, emailLogID, errorMessage)
	if err != nil {
		return fmt.Errorf("failed to update blast recipient status: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// BlastRecipientStats represents aggregate stats for blast recipients
type BlastRecipientStats struct {
	Total     int `db:"total"`
	Pending   int `db:"pending"`
	Queued    int `db:"queued"`
	Sending   int `db:"sending"`
	Sent      int `db:"sent"`
	Delivered int `db:"delivered"`
	Opened    int `db:"opened"`
	Clicked   int `db:"clicked"`
	Bounced   int `db:"bounced"`
	Failed    int `db:"failed"`
}

const sqlGetBlastRecipientStats = `
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE status = 'pending') as pending,
    COUNT(*) FILTER (WHERE status = 'queued') as queued,
    COUNT(*) FILTER (WHERE status = 'sending') as sending,
    COUNT(*) FILTER (WHERE status = 'sent') as sent,
    COUNT(*) FILTER (WHERE status = 'delivered') as delivered,
    COUNT(*) FILTER (WHERE status = 'opened') as opened,
    COUNT(*) FILTER (WHERE status = 'clicked') as clicked,
    COUNT(*) FILTER (WHERE status = 'bounced') as bounced,
    COUNT(*) FILTER (WHERE status = 'failed') as failed
FROM blast_recipients
WHERE blast_id = $1
`

// GetBlastRecipientStats retrieves aggregate stats for blast recipients
func (s *Store) GetBlastRecipientStats(ctx context.Context, blastID uuid.UUID) (BlastRecipientStats, error) {
	var stats BlastRecipientStats
	err := s.db.GetContext(ctx, &stats, sqlGetBlastRecipientStats, blastID)
	if err != nil {
		return BlastRecipientStats{}, fmt.Errorf("failed to get blast recipient stats: %w", err)
	}
	return stats, nil
}

const sqlGetMaxBatchNumber = `
SELECT COALESCE(MAX(batch_number), 0)
FROM blast_recipients
WHERE blast_id = $1
`

// GetMaxBatchNumber retrieves the maximum batch number for a blast
func (s *Store) GetMaxBatchNumber(ctx context.Context, blastID uuid.UUID) (int, error) {
	var maxBatch int
	err := s.db.GetContext(ctx, &maxBatch, sqlGetMaxBatchNumber, blastID)
	if err != nil {
		return 0, fmt.Errorf("failed to get max batch number: %w", err)
	}
	return maxBatch, nil
}

const sqlGetBlastRecipientsByBatch = `
SELECT id, blast_id, user_id, email, status, email_log_id, queued_at, sent_at, delivered_at, opened_at, clicked_at, bounced_at, failed_at, error_message, batch_number, created_at, updated_at
FROM blast_recipients
WHERE blast_id = $1 AND batch_number = $2
ORDER BY created_at ASC
`

// GetBlastRecipientsByBatch retrieves all recipients for a specific batch
func (s *Store) GetBlastRecipientsByBatch(ctx context.Context, blastID uuid.UUID, batchNumber int) ([]BlastRecipient, error) {
	var recipients []BlastRecipient
	err := s.db.SelectContext(ctx, &recipients, sqlGetBlastRecipientsByBatch, blastID, batchNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get blast recipients by batch: %w", err)
	}
	return recipients, nil
}

const sqlCountBlastRecipientsByStatus = `
SELECT COUNT(*)
FROM blast_recipients
WHERE blast_id = $1 AND status = $2
`

// CountBlastRecipientsByStatus counts recipients with a specific status
func (s *Store) CountBlastRecipientsByStatus(ctx context.Context, blastID uuid.UUID, status string) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count, sqlCountBlastRecipientsByStatus, blastID, status)
	if err != nil {
		return 0, fmt.Errorf("failed to count blast recipients by status: %w", err)
	}
	return count, nil
}

const sqlUpdateEmailBlastProgressWithSent = `
UPDATE email_blasts
SET sent_count = $2,
    current_batch = $3,
    last_batch_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND deleted_at IS NULL
`

// UpdateEmailBlastProgressWithSent updates sent count and batch progress
func (s *Store) UpdateEmailBlastProgressWithSent(ctx context.Context, blastID uuid.UUID, sentCount int, currentBatch int) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateEmailBlastProgressWithSent, blastID, sentCount, currentBatch)
	if err != nil {
		return fmt.Errorf("failed to update email blast progress: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// ScheduleBlast updates a blast to scheduled status with a scheduled time
func (s *Store) ScheduleBlast(ctx context.Context, blastID uuid.UUID, scheduledAt time.Time) (EmailBlast, error) {
	const query = `
	UPDATE email_blasts
	SET status = 'scheduled',
	    scheduled_at = $2,
	    updated_at = CURRENT_TIMESTAMP
	WHERE id = $1 AND deleted_at IS NULL AND status = 'draft'
	RETURNING id, campaign_id, segment_id, template_id, name, subject, scheduled_at, started_at, completed_at, status, total_recipients, sent_count, delivered_count, opened_count, clicked_count, bounced_count, failed_count, batch_size, current_batch, last_batch_at, error_message, send_throttle_per_second, created_by, created_at, updated_at, deleted_at
	`

	var blast EmailBlast
	err := s.db.GetContext(ctx, &blast, query, blastID, scheduledAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EmailBlast{}, ErrNotFound
		}
		return EmailBlast{}, fmt.Errorf("failed to schedule blast: %w", err)
	}
	return blast, nil
}
