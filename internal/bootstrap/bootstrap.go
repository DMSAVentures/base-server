package bootstrap

import (
	"base-server/internal/config"
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"fmt"
	"strings"
	"time"

	aiHandler "base-server/internal/ai-capabilities/handler"
	AICapabilities "base-server/internal/ai-capabilities/processor"
	analyticsHandler "base-server/internal/analytics/handler"
	analyticsProcessor "base-server/internal/analytics/processor"
	"base-server/internal/auth/handler"
	"base-server/internal/auth/processor"
	campaignHandler "base-server/internal/campaign/handler"
	campaignProcessor "base-server/internal/campaign/processor"
	"base-server/internal/clients/googleoauth"
	kafkaClient "base-server/internal/clients/kafka"
	"base-server/internal/clients/mail"
	"base-server/internal/email"
	emailTemplateHandler "base-server/internal/emailtemplates/handler"
	emailTemplateProcessor "base-server/internal/emailtemplates/processor"
	billingHandler "base-server/internal/money/billing/handler"
	billingProcessor "base-server/internal/money/billing/processor"
	"base-server/internal/money/products"
	"base-server/internal/money/subscriptions"
	referralHandler "base-server/internal/referral/handler"
	referralProcessor "base-server/internal/referral/processor"
	rewardHandler "base-server/internal/rewards/handler"
	rewardProcessor "base-server/internal/rewards/processor"
	voiceCallHandler "base-server/internal/voicecall/handler"
	voiceCallProcessor "base-server/internal/voicecall/processor"
	waitlistHandler "base-server/internal/waitlist/handler"
	waitlistProcessor "base-server/internal/waitlist/processor"
	"base-server/internal/webhooks/events"
	webhookHandler "base-server/internal/webhooks/handler"
	webhookEventProcessor "base-server/internal/webhooks/processor"
	webhookProducer "base-server/internal/webhooks/producer"
	webhookService "base-server/internal/webhooks/service"
	webhookWorker "base-server/internal/webhooks/worker"
	"base-server/internal/workers"
)

// Dependencies holds all initialized application dependencies
type Dependencies struct {
	// Core
	Store  store.Store
	Logger *observability.Logger

	// Handlers
	AuthHandler          handler.Handler
	BillingHandler       billingHandler.Handler
	AIHandler            aiHandler.Handler
	VoiceCallHandler     voiceCallHandler.Handler
	CampaignHandler      campaignHandler.Handler
	WaitlistHandler      waitlistHandler.Handler
	AnalyticsHandler     analyticsHandler.Handler
	ReferralHandler      referralHandler.Handler
	RewardHandler        rewardHandler.Handler
	EmailTemplateHandler emailTemplateHandler.Handler
	WebhookHandler       *webhookHandler.Handler

	// Background workers
	WebhookConsumer workers.EventConsumer
	EmailConsumer   workers.EventConsumer
	WebhookWorker   *webhookWorker.WebhookWorker

	// Kafka clients (for cleanup)
	KafkaProducer *kafkaClient.Producer
}

// Initialize sets up all application dependencies
func Initialize(ctx context.Context, cfg *config.Config, logger *observability.Logger) (*Dependencies, error) {
	deps := &Dependencies{
		Logger: logger,
	}

	// Initialize database store
	connectionString := cfg.Database.ConnectionString()
	var err error
	deps.Store, err = store.New(connectionString, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Initialize clients
	googleOAuthClient := googleoauth.NewClient(
		cfg.Auth.GoogleClientID,
		cfg.Auth.GoogleClientSecret,
		cfg.Auth.GoogleRedirectURI,
		logger,
	)

	mailClient, err := mail.NewResendClient(cfg.Services.ResendAPIKey, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create resend client: %w", err)
	}

	// Initialize email service
	emailService := email.New(mailClient, cfg.Services.DefaultEmailSender, logger)

	// Initialize Kafka producer
	brokerList := strings.Split(cfg.Kafka.Brokers, ",")
	deps.KafkaProducer = kafkaClient.NewProducer(kafkaClient.ProducerConfig{
		Brokers: brokerList,
		Topic:   cfg.Kafka.Topic,
	}, logger)

	// Initialize product and subscription services
	productService := products.New(cfg.Services.StripeSecretKey, deps.Store, logger)
	subscriptionService := subscriptions.New(logger, cfg.Services.StripeSecretKey, deps.Store)

	// Initialize billing processor and handler
	billingProc := billingProcessor.New(
		cfg.Services.StripeSecretKey,
		cfg.Services.StripeWebhookSecret,
		cfg.Services.WebAppURI,
		deps.Store,
		productService,
		subscriptionService,
		emailService,
		logger,
	)
	deps.BillingHandler = billingHandler.New(billingProc, logger)

	// Initialize auth processor and handler
	authConfig := processor.AuthConfig{
		Email: processor.EmailConfig{
			JWTSecret: cfg.Auth.JWTSecret,
		},
		Google: processor.GoogleOauthConfig{
			ClientID:          cfg.Auth.GoogleClientID,
			ClientSecret:      cfg.Auth.GoogleClientSecret,
			ClientRedirectURL: cfg.Auth.GoogleRedirectURI,
			WebAppHost:        cfg.Services.WebAppURI,
		},
	}
	authProc := processor.New(deps.Store, authConfig, googleOAuthClient, billingProc, *emailService, logger)
	deps.AuthHandler = handler.New(authProc, logger)

	// Initialize AI capabilities processor and handler
	aiCapability := AICapabilities.New(logger, cfg.Services.GoogleAIAPIKey, cfg.Services.OpenAIAPIKey, deps.Store)
	deps.AIHandler = aiHandler.New(aiCapability, logger)

	// Initialize voice call processor and handler
	voiceCallProc := voiceCallProcessor.NewVoiceCallProcessor(aiCapability, logger)
	deps.VoiceCallHandler = voiceCallHandler.New(voiceCallProc, logger)

	// Initialize event producer and dispatcher for internal events
	eventProducer := webhookProducer.New(deps.KafkaProducer, logger)
	eventDispatcher := events.NewEventDispatcher(eventProducer, logger)

	// Initialize campaign processor and handler
	campaignProc := campaignProcessor.New(deps.Store, logger)
	deps.CampaignHandler = campaignHandler.New(campaignProc, logger)

	// Initialize waitlist processor and handler
	waitlistProc := waitlistProcessor.New(deps.Store, logger, eventDispatcher)
	deps.WaitlistHandler = waitlistHandler.New(waitlistProc, logger, cfg.Services.WebAppURI)

	// Initialize analytics processor and handler
	analyticsProc := analyticsProcessor.New(deps.Store, logger)
	deps.AnalyticsHandler = analyticsHandler.New(analyticsProc, logger)

	// Initialize referral processor and handler
	referralProc := referralProcessor.New(deps.Store, logger)
	deps.ReferralHandler = referralHandler.New(referralProc, logger, cfg.Services.WebAppURI)

	// Initialize rewards processor and handler
	rewardProc := rewardProcessor.New(deps.Store, logger)
	deps.RewardHandler = rewardHandler.New(rewardProc, logger)

	// Initialize email template processor and handler
	emailTemplateProc := emailTemplateProcessor.New(deps.Store, emailService, logger)
	deps.EmailTemplateHandler = emailTemplateHandler.New(emailTemplateProc, logger)

	// Initialize webhook services
	webhookSvc := webhookService.New(&deps.Store, logger)
	webhookProc := webhookEventProcessor.New(&deps.Store, logger, webhookSvc)
	deps.WebhookHandler = webhookHandler.New(webhookProc, logger)

	// Initialize webhook retry worker (runs every 30 seconds)
	deps.WebhookWorker = webhookWorker.New(webhookSvc, logger, 30*time.Second)

	// Initialize webhook event processor and consumer with worker pool
	webhookEvtProcessor := webhookEventProcessor.NewWebhookEventProcessor(webhookSvc, logger)
	webhookConsumerConfig := workers.DefaultConsumerConfig(brokerList, cfg.Kafka.ConsumerGroup, cfg.Kafka.Topic)
	webhookConsumerConfig.WorkerPoolConfig.NumWorkers = cfg.WorkerPool.WebhookWorkers
	deps.WebhookConsumer = workers.NewConsumer(webhookConsumerConfig, webhookEvtProcessor, logger)

	// Initialize email event processor and consumer with worker pool
	emailEvtProcessor := email.NewEmailEventProcessor(emailService, deps.Store, logger)
	emailConsumerConfig := workers.DefaultConsumerConfig(brokerList, cfg.Kafka.ConsumerGroup+"-email", cfg.Kafka.Topic)
	emailConsumerConfig.WorkerPoolConfig.NumWorkers = cfg.WorkerPool.EmailWorkers
	deps.EmailConsumer = workers.NewConsumer(emailConsumerConfig, emailEvtProcessor, logger)

	return deps, nil
}

// Cleanup closes all resources that need cleanup
func (d *Dependencies) Cleanup() {
	if d.KafkaProducer != nil {
		d.KafkaProducer.Close()
	}
	// Note: Consumers manage their own Kafka reader cleanup in Stop()
}
