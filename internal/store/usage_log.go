package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type UsageLog struct {
	ID             uuid.UUID `db:"id"`
	UserID         uuid.UUID `db:"user_id"`
	ConversationID uuid.UUID `db:"conversation_id"`
	MessageID      uuid.UUID `db:"message_id"`
	TokensUsed     int       `db:"tokens_used"`
	CostInCents    int       `db:"cost_in_cents"`
	Model          string    `db:"model"`
	CreatedAt      time.Time `db:"created_at"`
}

const sqlInsertUsageLog = `
INSERT INTO usage_logs ( user_id, conversation_id, message_id, tokens_used, cost_in_cents, model)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, conversation_id, message_id, tokens_used, cost_in_cents, model, created_at
`

func (s *Store) InsertUsageLog(ctx context.Context, usageLog UsageLog) (UsageLog, error) {
	err := s.db.GetContext(ctx, &usageLog, sqlInsertUsageLog,
		usageLog.UserID,
		usageLog.ConversationID,
		usageLog.MessageID,
		usageLog.TokensUsed,
		usageLog.CostInCents,
		usageLog.Model,
	)
	if err != nil {
		s.logger.Error(ctx, "failed to insert usage log", err)
		return UsageLog{}, fmt.Errorf("failed to insert usage log: %w", err)
	}
	return usageLog, nil
}

const sqlGetUsageLogsByUserIDForPeriod = `
SELECT * FROM usage_logs WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
ORDER BY created_at DESC
`

func (s *Store) GetUsageLogsByUserIDForPeriod(ctx context.Context, userID uuid.UUID, startDate,
	endDate time.Time) ([]UsageLog, error) {
	var usageLogs []UsageLog
	err := s.db.SelectContext(ctx, &usageLogs, sqlGetUsageLogsByUserIDForPeriod,
		userID,
		startDate,
		endDate,
	)
	if err != nil {
		s.logger.Error(ctx, "failed to get usage logs by user ID for period", err)
		return nil, fmt.Errorf("failed to get usage logs by user ID for period: %w", err)
	}
	return usageLogs, nil
}
