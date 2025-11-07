package kafka

// Topic definitions for the waitlist platform
// Kafka topics provide durability, replay capability, and event sourcing

const (
	// High priority topics - Critical business events
	TopicEmailVerification   = "waitlist.email.verification"
	TopicEmailWelcome        = "waitlist.email.welcome"
	TopicEmailPositionUpdate = "waitlist.email.position-update"
	TopicEmailRewardEarned   = "waitlist.email.reward-earned"
	TopicEmailMilestone      = "waitlist.email.milestone"
	TopicRewardDelivery      = "waitlist.reward.delivery"

	// Medium priority topics - Important but not time-critical
	TopicPositionRecalc      = "waitlist.position.recalculation"
	TopicFraudDetection      = "waitlist.fraud.detection"
	TopicWebhookDelivery     = "waitlist.webhook.delivery"
	TopicAnalyticsAggregation = "waitlist.analytics.aggregation"

	// Low priority topics - Background processing
	TopicEmailTracking = "waitlist.email.tracking"
	TopicDataExport    = "waitlist.data.export"

	// Dead letter queue topic - Failed messages
	TopicDeadLetter = "waitlist.dlq"

	// Event sourcing topics - Audit trail
	TopicUserSignup      = "waitlist.events.user.signup"
	TopicUserVerified    = "waitlist.events.user.verified"
	TopicReferralCreated = "waitlist.events.referral.created"
	TopicRewardEarned    = "waitlist.events.reward.earned"
)

// TopicConfig represents Kafka topic configuration
type TopicConfig struct {
	Name              string
	NumPartitions     int
	ReplicationFactor int
	RetentionHours    int // How long to retain messages
	Description       string
}

// GetTopicConfigs returns all topic configurations
func GetTopicConfigs() []TopicConfig {
	return []TopicConfig{
		// High priority - Job topics
		{
			Name:              TopicEmailVerification,
			NumPartitions:     10, // High parallelism
			ReplicationFactor: 3,  // High durability
			RetentionHours:    168, // 7 days
			Description:       "Email verification sending jobs",
		},
		{
			Name:              TopicEmailWelcome,
			NumPartitions:     10,
			ReplicationFactor: 3,
			RetentionHours:    168,
			Description:       "Welcome email sending jobs",
		},
		{
			Name:              TopicEmailPositionUpdate,
			NumPartitions:     10,
			ReplicationFactor: 3,
			RetentionHours:    168,
			Description:       "Position update notification jobs",
		},
		{
			Name:              TopicEmailRewardEarned,
			NumPartitions:     10,
			ReplicationFactor: 3,
			RetentionHours:    168,
			Description:       "Reward earned email jobs",
		},
		{
			Name:              TopicEmailMilestone,
			NumPartitions:     5,
			ReplicationFactor: 3,
			RetentionHours:    168,
			Description:       "Milestone email jobs",
		},
		{
			Name:              TopicRewardDelivery,
			NumPartitions:     10,
			ReplicationFactor: 3,
			RetentionHours:    720, // 30 days - rewards are critical
			Description:       "Reward delivery jobs",
		},

		// Medium priority - Processing topics
		{
			Name:              TopicPositionRecalc,
			NumPartitions:     5,
			ReplicationFactor: 3,
			RetentionHours:    168,
			Description:       "Position recalculation jobs",
		},
		{
			Name:              TopicFraudDetection,
			NumPartitions:     5,
			ReplicationFactor: 3,
			RetentionHours:    720, // 30 days - fraud history important
			Description:       "Fraud detection analysis jobs",
		},
		{
			Name:              TopicWebhookDelivery,
			NumPartitions:     5,
			ReplicationFactor: 3,
			RetentionHours:    168,
			Description:       "Webhook delivery jobs",
		},
		{
			Name:              TopicAnalyticsAggregation,
			NumPartitions:     3,
			ReplicationFactor: 3,
			RetentionHours:    168,
			Description:       "Analytics aggregation jobs",
		},

		// Low priority topics
		{
			Name:              TopicEmailTracking,
			NumPartitions:     3,
			ReplicationFactor: 2,
			RetentionHours:    72, // 3 days
			Description:       "Email open/click tracking",
		},
		{
			Name:              TopicDataExport,
			NumPartitions:     2,
			ReplicationFactor: 2,
			RetentionHours:    72,
			Description:       "Data export jobs",
		},

		// Dead letter queue
		{
			Name:              TopicDeadLetter,
			NumPartitions:     3,
			ReplicationFactor: 3,
			RetentionHours:    8760, // 1 year - keep failed messages for debugging
			Description:       "Failed messages from all topics",
		},

		// Event sourcing topics - These are append-only audit logs
		{
			Name:              TopicUserSignup,
			NumPartitions:     10,
			ReplicationFactor: 3,
			RetentionHours:    8760, // 1 year - compliance/audit
			Description:       "User signup events",
		},
		{
			Name:              TopicUserVerified,
			NumPartitions:     10,
			ReplicationFactor: 3,
			RetentionHours:    8760, // 1 year
			Description:       "User email verified events",
		},
		{
			Name:              TopicReferralCreated,
			NumPartitions:     10,
			ReplicationFactor: 3,
			RetentionHours:    8760, // 1 year
			Description:       "Referral created events",
		},
		{
			Name:              TopicRewardEarned,
			NumPartitions:     10,
			ReplicationFactor: 3,
			RetentionHours:    8760, // 1 year
			Description:       "Reward earned events",
		},
	}
}

// Consumer group IDs
const (
	ConsumerGroupEmailWorkers     = "email-workers"
	ConsumerGroupPositionWorkers  = "position-workers"
	ConsumerGroupRewardWorkers    = "reward-workers"
	ConsumerGroupAnalyticsWorkers = "analytics-workers"
	ConsumerGroupFraudWorkers     = "fraud-workers"
	ConsumerGroupWebhookWorkers   = "webhook-workers"
	ConsumerGroupEventProcessor   = "event-processor" // For event sourcing
)
