package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateAPIKeyParams represents parameters for creating an API key
type CreateAPIKeyParams struct {
	AccountID     uuid.UUID
	Name          string
	KeyHash       string
	KeyPrefix     string
	Scopes        []string
	RateLimitTier string
	ExpiresAt     *time.Time
	CreatedBy     *uuid.UUID
}

const sqlCreateAPIKey = `
INSERT INTO api_keys (account_id, name, key_hash, key_prefix, scopes, rate_limit_tier, expires_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, account_id, name, key_hash, key_prefix, scopes, rate_limit_tier, status, last_used_at, total_requests, expires_at, created_by, created_at, revoked_at, revoked_by
`

// CreateAPIKey creates a new API key
func (s *Store) CreateAPIKey(ctx context.Context, params CreateAPIKeyParams) (APIKey, error) {
	var apiKey APIKey
	err := s.db.GetContext(ctx, &apiKey, sqlCreateAPIKey,
		params.AccountID,
		params.Name,
		params.KeyHash,
		params.KeyPrefix,
		StringArray(params.Scopes),
		params.RateLimitTier,
		params.ExpiresAt,
		params.CreatedBy)
	if err != nil {
		s.logger.Error(ctx, "failed to create api key", err)
		return APIKey{}, fmt.Errorf("failed to create api key: %w", err)
	}
	return apiKey, nil
}

const sqlGetAPIKeyByHash = `
SELECT id, account_id, name, key_hash, key_prefix, scopes, rate_limit_tier, status, last_used_at, total_requests, expires_at, created_by, created_at, revoked_at, revoked_by
FROM api_keys
WHERE key_hash = $1 AND status = 'active' AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
`

// GetAPIKeyByHash retrieves an API key by its hash
func (s *Store) GetAPIKeyByHash(ctx context.Context, keyHash string) (APIKey, error) {
	var apiKey APIKey
	err := s.db.GetContext(ctx, &apiKey, sqlGetAPIKeyByHash, keyHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIKey{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get api key by hash", err)
		return APIKey{}, fmt.Errorf("failed to get api key by hash: %w", err)
	}
	return apiKey, nil
}

const sqlGetAPIKeyByID = `
SELECT id, account_id, name, key_hash, key_prefix, scopes, rate_limit_tier, status, last_used_at, total_requests, expires_at, created_by, created_at, revoked_at, revoked_by
FROM api_keys
WHERE id = $1
`

// GetAPIKeyByID retrieves an API key by ID
func (s *Store) GetAPIKeyByID(ctx context.Context, keyID uuid.UUID) (APIKey, error) {
	var apiKey APIKey
	err := s.db.GetContext(ctx, &apiKey, sqlGetAPIKeyByID, keyID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return APIKey{}, ErrNotFound
		}
		s.logger.Error(ctx, "failed to get api key by id", err)
		return APIKey{}, fmt.Errorf("failed to get api key by id: %w", err)
	}
	return apiKey, nil
}

const sqlGetAPIKeysByAccount = `
SELECT id, account_id, name, key_hash, key_prefix, scopes, rate_limit_tier, status, last_used_at, total_requests, expires_at, created_by, created_at, revoked_at, revoked_by
FROM api_keys
WHERE account_id = $1
ORDER BY created_at DESC
`

// GetAPIKeysByAccount retrieves all API keys for an account
func (s *Store) GetAPIKeysByAccount(ctx context.Context, accountID uuid.UUID) ([]APIKey, error) {
	var apiKeys []APIKey
	err := s.db.SelectContext(ctx, &apiKeys, sqlGetAPIKeysByAccount, accountID)
	if err != nil {
		s.logger.Error(ctx, "failed to get api keys by account", err)
		return nil, fmt.Errorf("failed to get api keys by account: %w", err)
	}
	return apiKeys, nil
}

const sqlUpdateAPIKeyUsage = `
UPDATE api_keys
SET last_used_at = CURRENT_TIMESTAMP,
    total_requests = total_requests + 1
WHERE id = $1
`

// UpdateAPIKeyUsage updates the last used timestamp and increments request count
func (s *Store) UpdateAPIKeyUsage(ctx context.Context, keyID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, sqlUpdateAPIKeyUsage, keyID)
	if err != nil {
		s.logger.Error(ctx, "failed to update api key usage", err)
		return fmt.Errorf("failed to update api key usage: %w", err)
	}
	return nil
}

const sqlRevokeAPIKey = `
UPDATE api_keys
SET status = 'revoked',
    revoked_at = CURRENT_TIMESTAMP,
    revoked_by = $2
WHERE id = $1 AND status = 'active'
`

// RevokeAPIKey revokes an API key
func (s *Store) RevokeAPIKey(ctx context.Context, keyID uuid.UUID, revokedBy uuid.UUID) error {
	res, err := s.db.ExecContext(ctx, sqlRevokeAPIKey, keyID, revokedBy)
	if err != nil {
		s.logger.Error(ctx, "failed to revoke api key", err)
		return fmt.Errorf("failed to revoke api key: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.logger.Error(ctx, "failed to get rows affected", err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlUpdateAPIKeyName = `
UPDATE api_keys
SET name = $2
WHERE id = $1
`

// UpdateAPIKeyName updates the name of an API key
func (s *Store) UpdateAPIKeyName(ctx context.Context, keyID uuid.UUID, name string) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateAPIKeyName, keyID, name)
	if err != nil {
		s.logger.Error(ctx, "failed to update api key name", err)
		return fmt.Errorf("failed to update api key name: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.logger.Error(ctx, "failed to get rows affected", err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// Audit Log operations

// CreateAuditLogParams represents parameters for creating an audit log
type CreateAuditLogParams struct {
	AccountID       *uuid.UUID
	ActorUserID     *uuid.UUID
	ActorType       string
	ActorIdentifier *string
	Action          string
	ResourceType    string
	ResourceID      *uuid.UUID
	Changes         JSONB
	IPAddress       *string
	UserAgent       *string
}

const sqlCreateAuditLog = `
INSERT INTO audit_logs (account_id, actor_user_id, actor_type, actor_identifier, action, resource_type, resource_id, changes, ip_address, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, account_id, actor_user_id, actor_type, actor_identifier, action, resource_type, resource_id, changes, ip_address, user_agent, created_at
`

// CreateAuditLog creates a new audit log entry
func (s *Store) CreateAuditLog(ctx context.Context, params CreateAuditLogParams) (AuditLog, error) {
	var log AuditLog
	err := s.db.GetContext(ctx, &log, sqlCreateAuditLog,
		params.AccountID,
		params.ActorUserID,
		params.ActorType,
		params.ActorIdentifier,
		params.Action,
		params.ResourceType,
		params.ResourceID,
		params.Changes,
		params.IPAddress,
		params.UserAgent)
	if err != nil {
		s.logger.Error(ctx, "failed to create audit log", err)
		return AuditLog{}, fmt.Errorf("failed to create audit log: %w", err)
	}
	return log, nil
}

const sqlGetAuditLogsByAccount = `
SELECT id, account_id, actor_user_id, actor_type, actor_identifier, action, resource_type, resource_id, changes, ip_address, user_agent, created_at
FROM audit_logs
WHERE account_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
`

// GetAuditLogsByAccount retrieves audit logs for an account with pagination
func (s *Store) GetAuditLogsByAccount(ctx context.Context, accountID uuid.UUID, limit, offset int) ([]AuditLog, error) {
	var logs []AuditLog
	err := s.db.SelectContext(ctx, &logs, sqlGetAuditLogsByAccount, accountID, limit, offset)
	if err != nil {
		s.logger.Error(ctx, "failed to get audit logs by account", err)
		return nil, fmt.Errorf("failed to get audit logs by account: %w", err)
	}
	return logs, nil
}

const sqlGetAuditLogsByResource = `
SELECT id, account_id, actor_user_id, actor_type, actor_identifier, action, resource_type, resource_id, changes, ip_address, user_agent, created_at
FROM audit_logs
WHERE resource_type = $1 AND resource_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4
`

// GetAuditLogsByResource retrieves audit logs for a specific resource
func (s *Store) GetAuditLogsByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, limit, offset int) ([]AuditLog, error) {
	var logs []AuditLog
	err := s.db.SelectContext(ctx, &logs, sqlGetAuditLogsByResource, resourceType, resourceID, limit, offset)
	if err != nil {
		s.logger.Error(ctx, "failed to get audit logs by resource", err)
		return nil, fmt.Errorf("failed to get audit logs by resource: %w", err)
	}
	return logs, nil
}

// Fraud Detection operations

// CreateFraudDetectionParams represents parameters for creating a fraud detection
type CreateFraudDetectionParams struct {
	CampaignID      uuid.UUID
	UserID          *uuid.UUID
	DetectionType   string
	ConfidenceScore float64
	Details         JSONB
}

const sqlCreateFraudDetection = `
INSERT INTO fraud_detections (campaign_id, user_id, detection_type, confidence_score, details)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, campaign_id, user_id, detection_type, confidence_score, details, status, reviewed_by, reviewed_at, review_notes, created_at
`

// CreateFraudDetection creates a new fraud detection record
func (s *Store) CreateFraudDetection(ctx context.Context, params CreateFraudDetectionParams) (FraudDetection, error) {
	var detection FraudDetection
	err := s.db.GetContext(ctx, &detection, sqlCreateFraudDetection,
		params.CampaignID,
		params.UserID,
		params.DetectionType,
		params.ConfidenceScore,
		params.Details)
	if err != nil {
		s.logger.Error(ctx, "failed to create fraud detection", err)
		return FraudDetection{}, fmt.Errorf("failed to create fraud detection: %w", err)
	}
	return detection, nil
}

const sqlGetFraudDetectionsByCampaign = `
SELECT id, campaign_id, user_id, detection_type, confidence_score, details, status, reviewed_by, reviewed_at, review_notes, created_at
FROM fraud_detections
WHERE campaign_id = $1
ORDER BY confidence_score DESC, created_at DESC
LIMIT $2 OFFSET $3
`

// GetFraudDetectionsByCampaign retrieves fraud detections for a campaign
func (s *Store) GetFraudDetectionsByCampaign(ctx context.Context, campaignID uuid.UUID, limit, offset int) ([]FraudDetection, error) {
	var detections []FraudDetection
	err := s.db.SelectContext(ctx, &detections, sqlGetFraudDetectionsByCampaign, campaignID, limit, offset)
	if err != nil {
		s.logger.Error(ctx, "failed to get fraud detections by campaign", err)
		return nil, fmt.Errorf("failed to get fraud detections by campaign: %w", err)
	}
	return detections, nil
}

const sqlGetFraudDetectionsByUser = `
SELECT id, campaign_id, user_id, detection_type, confidence_score, details, status, reviewed_by, reviewed_at, review_notes, created_at
FROM fraud_detections
WHERE user_id = $1
ORDER BY created_at DESC
`

// GetFraudDetectionsByUser retrieves fraud detections for a user
func (s *Store) GetFraudDetectionsByUser(ctx context.Context, userID uuid.UUID) ([]FraudDetection, error) {
	var detections []FraudDetection
	err := s.db.SelectContext(ctx, &detections, sqlGetFraudDetectionsByUser, userID)
	if err != nil {
		s.logger.Error(ctx, "failed to get fraud detections by user", err)
		return nil, fmt.Errorf("failed to get fraud detections by user: %w", err)
	}
	return detections, nil
}

const sqlUpdateFraudDetectionStatus = `
UPDATE fraud_detections
SET status = $2,
    reviewed_by = $3,
    reviewed_at = CURRENT_TIMESTAMP,
    review_notes = $4
WHERE id = $1
`

// UpdateFraudDetectionStatus updates the status of a fraud detection
func (s *Store) UpdateFraudDetectionStatus(ctx context.Context, detectionID, reviewedBy uuid.UUID, status string, reviewNotes *string) error {
	res, err := s.db.ExecContext(ctx, sqlUpdateFraudDetectionStatus, detectionID, status, reviewedBy, reviewNotes)
	if err != nil {
		s.logger.Error(ctx, "failed to update fraud detection status", err)
		return fmt.Errorf("failed to update fraud detection status: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		s.logger.Error(ctx, "failed to get rows affected", err)
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

const sqlGetPendingFraudDetections = `
SELECT id, campaign_id, user_id, detection_type, confidence_score, details, status, reviewed_by, reviewed_at, review_notes, created_at
FROM fraud_detections
WHERE status = 'pending' AND confidence_score >= $1
ORDER BY confidence_score DESC, created_at DESC
LIMIT $2
`

// GetPendingFraudDetections retrieves pending fraud detections above a confidence threshold
func (s *Store) GetPendingFraudDetections(ctx context.Context, minConfidence float64, limit int) ([]FraudDetection, error) {
	var detections []FraudDetection
	err := s.db.SelectContext(ctx, &detections, sqlGetPendingFraudDetections, minConfidence, limit)
	if err != nil {
		s.logger.Error(ctx, "failed to get pending fraud detections", err)
		return nil, fmt.Errorf("failed to get pending fraud detections: %w", err)
	}
	return detections, nil
}
