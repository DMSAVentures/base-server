package jobs

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// Job type constants
const (
	// High priority queue
	TypeEmailVerification      = "email:verification"
	TypeEmailWelcome           = "email:welcome"
	TypeEmailPositionUpdate    = "email:position_update"
	TypeEmailRewardEarned      = "email:reward_earned"
	TypeEmailMilestone         = "email:milestone"
	TypeRewardDelivery         = "reward:delivery"

	// Medium priority queue
	TypePositionRecalculation  = "position:recalculation"
	TypeFraudDetection         = "fraud:detection"
	TypeWebhookDelivery        = "webhook:delivery"
	TypeAnalyticsAggregation   = "analytics:aggregation"

	// Low priority queue
	TypeEmailTracking          = "email:tracking"
	TypeDataExport             = "data:export"
)

// Queue names
const (
	QueueHigh   = "high"
	QueueMedium = "medium"
	QueueLow    = "low"
)

// EmailJobPayload represents a generic email job
type EmailJobPayload struct {
	Type         string                 `json:"type"`          // verification, welcome, reward, etc.
	CampaignID   uuid.UUID              `json:"campaign_id"`
	UserID       uuid.UUID              `json:"user_id"`
	TemplateID   *uuid.UUID             `json:"template_id,omitempty"`
	TemplateData map[string]interface{} `json:"template_data"`
	Priority     int                    `json:"priority"`
}

// NewEmailTask creates a new email task
func NewEmailTask(payload EmailJobPayload, queue string) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var taskType string
	switch payload.Type {
	case "verification":
		taskType = TypeEmailVerification
	case "welcome":
		taskType = TypeEmailWelcome
	case "position_update":
		taskType = TypeEmailPositionUpdate
	case "reward_earned":
		taskType = TypeEmailRewardEarned
	case "milestone":
		taskType = TypeEmailMilestone
	default:
		taskType = TypeEmailVerification
	}

	return asynq.NewTask(taskType, data, asynq.Queue(queue), asynq.MaxRetry(5)), nil
}

// PositionRecalcJobPayload represents a position recalculation job
type PositionRecalcJobPayload struct {
	CampaignID uuid.UUID    `json:"campaign_id"`
	UserIDs    []uuid.UUID  `json:"user_ids,omitempty"` // specific users to recalc, or nil for all
}

// NewPositionRecalcTask creates a new position recalculation task
func NewPositionRecalcTask(payload PositionRecalcJobPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypePositionRecalculation, data, asynq.Queue(QueueMedium), asynq.MaxRetry(3)), nil
}

// RewardDeliveryJobPayload represents a reward delivery job
type RewardDeliveryJobPayload struct {
	UserRewardID uuid.UUID `json:"user_reward_id"`
	RetryAttempt int       `json:"retry_attempt"`
}

// NewRewardDeliveryTask creates a new reward delivery task
func NewRewardDeliveryTask(payload RewardDeliveryJobPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeRewardDelivery, data, asynq.Queue(QueueHigh), asynq.MaxRetry(5)), nil
}

// WebhookDeliveryJobPayload represents a webhook delivery job
type WebhookDeliveryJobPayload struct {
	WebhookID  uuid.UUID              `json:"webhook_id"`
	EventType  string                 `json:"event_type"`
	Payload    map[string]interface{} `json:"payload"`
	Attempt    int                    `json:"attempt"`
	MaxRetries int                    `json:"max_retries"`
}

// NewWebhookDeliveryTask creates a new webhook delivery task
func NewWebhookDeliveryTask(payload WebhookDeliveryJobPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeWebhookDelivery, data, asynq.Queue(QueueMedium), asynq.MaxRetry(5)), nil
}

// FraudDetectionJobPayload represents a fraud detection job
type FraudDetectionJobPayload struct {
	CampaignID uuid.UUID `json:"campaign_id"`
	UserID     uuid.UUID `json:"user_id"`
	CheckTypes []string  `json:"check_types"` // self_referral, velocity, fake_email, etc.
}

// NewFraudDetectionTask creates a new fraud detection task
func NewFraudDetectionTask(payload FraudDetectionJobPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeFraudDetection, data, asynq.Queue(QueueMedium), asynq.MaxRetry(3)), nil
}

// AnalyticsAggregationJobPayload represents an analytics aggregation job
type AnalyticsAggregationJobPayload struct {
	CampaignID  uuid.UUID `json:"campaign_id"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Granularity string    `json:"granularity"` // hour, day, week, month
}

// NewAnalyticsAggregationTask creates a new analytics aggregation task
func NewAnalyticsAggregationTask(payload AnalyticsAggregationJobPayload) (*asynq.Task, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeAnalyticsAggregation, data, asynq.Queue(QueueMedium), asynq.MaxRetry(3)), nil
}
