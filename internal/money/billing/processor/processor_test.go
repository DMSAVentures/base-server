package processor

import (
	"base-server/internal/money/products"
	"base-server/internal/money/subscriptions"
	"base-server/internal/observability"
	"base-server/internal/store"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v79"
	"go.uber.org/mock/gomock"
)

func TestNew(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	if processor.stripKey != "test_stripe_key" {
		t.Errorf("expected stripKey to be 'test_stripe_key', got %s", processor.stripKey)
	}
	if processor.WebhookSecret != "test_webhook_secret" {
		t.Errorf("expected WebhookSecret to be 'test_webhook_secret', got %s", processor.WebhookSecret)
	}
	if processor.webhostURL != "http://localhost:3000" {
		t.Errorf("expected webhostURL to be 'http://localhost:3000', got %s", processor.webhostURL)
	}
}

func TestGetPaymentMethodForUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	ctx := context.Background()
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		expectedPaymentMethod := &store.PaymentMethod{
			ID:           uuid.New(),
			UserID:       userID,
			StripeID:     "pm_test123",
			CardBrand:    "visa",
			CardLast4:    "4242",
			CardExpMonth: 12,
			CardExpYear:  2025,
		}

		mockStore.EXPECT().
			GetPaymentMethodByUserID(gomock.Any(), userID).
			Return(expectedPaymentMethod, nil)

		result, err := processor.GetPaymentMethodForUser(ctx, userID)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result.ID != expectedPaymentMethod.ID {
			t.Errorf("expected ID %v, got %v", expectedPaymentMethod.ID, result.ID)
		}
		if result.CardBrand != expectedPaymentMethod.CardBrand {
			t.Errorf("expected CardBrand %s, got %s", expectedPaymentMethod.CardBrand, result.CardBrand)
		}
		if result.CardLast4 != expectedPaymentMethod.CardLast4 {
			t.Errorf("expected CardLast4 %s, got %s", expectedPaymentMethod.CardLast4, result.CardLast4)
		}
		if result.CardExpMonth != expectedPaymentMethod.CardExpMonth {
			t.Errorf("expected CardExpMonth %d, got %d", expectedPaymentMethod.CardExpMonth, result.CardExpMonth)
		}
		if result.CardExpYear != expectedPaymentMethod.CardExpYear {
			t.Errorf("expected CardExpYear %d, got %d", expectedPaymentMethod.CardExpYear, result.CardExpYear)
		}
	})

	t.Run("store error", func(t *testing.T) {
		mockStore.EXPECT().
			GetPaymentMethodByUserID(gomock.Any(), userID).
			Return(nil, errors.New("database error"))

		_, err := processor.GetPaymentMethodForUser(ctx, userID)

		if err == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(err, ErrFailedToGetPaymentMethod) {
			t.Errorf("expected ErrFailedToGetPaymentMethod, got %v", err)
		}
	})
}

func TestListPrices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expectedPrices := []products.Price{
			{
				ProductID:   uuid.New(),
				PriceID:     uuid.New(),
				Description: "Basic Plan",
			},
			{
				ProductID:   uuid.New(),
				PriceID:     uuid.New(),
				Description: "Pro Plan",
			},
		}

		mockProductService.EXPECT().
			ListPrices(gomock.Any()).
			Return(expectedPrices, nil)

		result, err := processor.ListPrices(ctx)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if len(result) != len(expectedPrices) {
			t.Errorf("expected %d prices, got %d", len(expectedPrices), len(result))
		}
		for i, price := range result {
			if price.ProductID != expectedPrices[i].ProductID {
				t.Errorf("expected ProductID %v, got %v", expectedPrices[i].ProductID, price.ProductID)
			}
			if price.Description != expectedPrices[i].Description {
				t.Errorf("expected Description %s, got %s", expectedPrices[i].Description, price.Description)
			}
		}
	})

	t.Run("service error", func(t *testing.T) {
		mockProductService.EXPECT().
			ListPrices(gomock.Any()).
			Return(nil, errors.New("service error"))

		_, err := processor.ListPrices(ctx)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		mockProductService.EXPECT().
			ListPrices(gomock.Any()).
			Return([]products.Price{}, nil)

		result, err := processor.ListPrices(ctx)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected 0 prices, got %d", len(result))
		}
	})
}

func TestGetActiveSubscription(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	ctx := context.Background()
	userID := uuid.New()

	t.Run("success", func(t *testing.T) {
		expectedSub := subscriptions.Subscription{
			ID:              uuid.New(),
			UserID:          userID,
			PriceID:         uuid.New(),
			StripeID:        "sub_test123",
			Status:          "active",
			StartDate:       time.Now().Add(-30 * 24 * time.Hour),
			EndDate:         time.Now().Add(30 * 24 * time.Hour),
			NextBillingDate: time.Now().Add(30 * 24 * time.Hour),
		}

		mockSubscriptionService.EXPECT().
			GetSubscriptionByUserID(gomock.Any(), userID).
			Return(expectedSub, nil)

		result, err := processor.GetActiveSubscription(ctx, userID)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if result.ID != expectedSub.ID {
			t.Errorf("expected ID %v, got %v", expectedSub.ID, result.ID)
		}
		if result.StripeID != expectedSub.StripeID {
			t.Errorf("expected StripeID %s, got %s", expectedSub.StripeID, result.StripeID)
		}
		if result.Status != expectedSub.Status {
			t.Errorf("expected Status %s, got %s", expectedSub.Status, result.Status)
		}
	})

	t.Run("no subscription found", func(t *testing.T) {
		mockSubscriptionService.EXPECT().
			GetSubscriptionByUserID(gomock.Any(), userID).
			Return(subscriptions.Subscription{}, subscriptions.ErrNoSubscription)

		_, err := processor.GetActiveSubscription(ctx, userID)

		if err == nil {
			t.Error("expected error, got nil")
		}
		if !errors.Is(err, ErrNoActiveSubscription) {
			t.Errorf("expected ErrNoActiveSubscription, got %v", err)
		}
	})

	t.Run("service error", func(t *testing.T) {
		mockSubscriptionService.EXPECT().
			GetSubscriptionByUserID(gomock.Any(), userID).
			Return(subscriptions.Subscription{}, errors.New("database error"))

		_, err := processor.GetActiveSubscription(ctx, userID)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestProductCreated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		product := stripe.Product{
			ID:          "prod_test123",
			Name:        "Test Product",
			Description: "A test product",
		}

		mockProductService.EXPECT().
			CreateProduct(gomock.Any(), product).
			Return(nil)

		err := processor.ProductCreated(ctx, product)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("service error", func(t *testing.T) {
		product := stripe.Product{
			ID:          "prod_test123",
			Name:        "Test Product",
			Description: "A test product",
		}

		mockProductService.EXPECT().
			CreateProduct(gomock.Any(), product).
			Return(errors.New("service error"))

		err := processor.ProductCreated(ctx, product)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestPriceCreated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		price := stripe.Price{
			ID:       "price_test123",
			Nickname: "Monthly Plan",
			Product: &stripe.Product{
				ID: "prod_test123",
			},
		}

		mockProductService.EXPECT().
			CreatePrice(gomock.Any(), price).
			Return(nil)

		err := processor.PriceCreated(ctx, price)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("service error", func(t *testing.T) {
		price := stripe.Price{
			ID:       "price_test123",
			Nickname: "Monthly Plan",
			Product: &stripe.Product{
				ID: "prod_test123",
			},
		}

		mockProductService.EXPECT().
			CreatePrice(gomock.Any(), price).
			Return(errors.New("service error"))

		err := processor.PriceCreated(ctx, price)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestPriceUpdated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		price := stripe.Price{
			ID:       "price_test123",
			Nickname: "Updated Monthly Plan",
			Product: &stripe.Product{
				ID: "prod_test123",
			},
		}

		mockProductService.EXPECT().
			UpdatePrice(gomock.Any(), price).
			Return(nil)

		err := processor.PriceUpdated(ctx, price)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("service error", func(t *testing.T) {
		price := stripe.Price{
			ID:       "price_test123",
			Nickname: "Updated Monthly Plan",
			Product: &stripe.Product{
				ID: "prod_test123",
			},
		}

		mockProductService.EXPECT().
			UpdatePrice(gomock.Any(), price).
			Return(errors.New("service error"))

		err := processor.PriceUpdated(ctx, price)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestPriceDeleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		price := stripe.Price{
			ID: "price_test123",
		}

		mockProductService.EXPECT().
			DeletePrice(gomock.Any(), price).
			Return(nil)

		err := processor.PriceDeleted(ctx, price)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("service error", func(t *testing.T) {
		price := stripe.Price{
			ID: "price_test123",
		}

		mockProductService.EXPECT().
			DeletePrice(gomock.Any(), price).
			Return(errors.New("service error"))

		err := processor.PriceDeleted(ctx, price)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestSubscriptionCreated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		subscription := stripe.Subscription{
			ID:     "sub_test123",
			Status: stripe.SubscriptionStatusActive,
			Customer: &stripe.Customer{
				ID: "cus_test123",
			},
			Items: &stripe.SubscriptionItemList{
				Data: []*stripe.SubscriptionItem{
					{
						Price: &stripe.Price{
							ID: "price_test123",
						},
					},
				},
			},
			CurrentPeriodStart: time.Now().Unix(),
			CurrentPeriodEnd:   time.Now().Add(30 * 24 * time.Hour).Unix(),
		}

		mockSubscriptionService.EXPECT().
			CreateSubscription(gomock.Any(), subscription).
			Return(nil)

		err := processor.SubscriptionCreated(ctx, subscription)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("service error", func(t *testing.T) {
		subscription := stripe.Subscription{
			ID:     "sub_test123",
			Status: stripe.SubscriptionStatusActive,
		}

		mockSubscriptionService.EXPECT().
			CreateSubscription(gomock.Any(), subscription).
			Return(errors.New("service error"))

		err := processor.SubscriptionCreated(ctx, subscription)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestSubscriptionUpdated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	ctx := context.Background()

	t.Run("success without cancellation", func(t *testing.T) {
		subscription := stripe.Subscription{
			ID:       "sub_test123",
			Status:   stripe.SubscriptionStatusActive,
			CancelAt: 0,
			Items: &stripe.SubscriptionItemList{
				Data: []*stripe.SubscriptionItem{
					{
						Price: &stripe.Price{
							ID: "price_test123",
						},
					},
				},
			},
			CurrentPeriodEnd: time.Now().Add(30 * 24 * time.Hour).Unix(),
		}

		mockSubscriptionService.EXPECT().
			UpdateSubscription(gomock.Any(), subscription).
			Return(nil)

		err := processor.SubscriptionUpdated(ctx, subscription)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("success with cancellation scheduled", func(t *testing.T) {
		cancelAt := time.Now().Add(30 * 24 * time.Hour).Unix()
		subscription := stripe.Subscription{
			ID:       "sub_test123",
			Status:   stripe.SubscriptionStatusActive,
			CancelAt: cancelAt,
		}

		mockSubscriptionService.EXPECT().
			CancelSubscription(gomock.Any(), subscription.ID, gomock.Any()).
			Return(nil)

		err := processor.SubscriptionUpdated(ctx, subscription)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("service error on update", func(t *testing.T) {
		subscription := stripe.Subscription{
			ID:       "sub_test123",
			Status:   stripe.SubscriptionStatusActive,
			CancelAt: 0,
		}

		mockSubscriptionService.EXPECT().
			UpdateSubscription(gomock.Any(), subscription).
			Return(errors.New("service error"))

		err := processor.SubscriptionUpdated(ctx, subscription)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("service error on cancellation", func(t *testing.T) {
		cancelAt := time.Now().Add(30 * 24 * time.Hour).Unix()
		subscription := stripe.Subscription{
			ID:       "sub_test123",
			Status:   stripe.SubscriptionStatusActive,
			CancelAt: cancelAt,
		}

		mockSubscriptionService.EXPECT().
			CancelSubscription(gomock.Any(), subscription.ID, gomock.Any()).
			Return(errors.New("service error"))

		err := processor.SubscriptionUpdated(ctx, subscription)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestSubscriptionDeleted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		subscription := stripe.Subscription{
			ID:     "sub_test123",
			Status: stripe.SubscriptionStatusCanceled,
		}

		mockSubscriptionService.EXPECT().
			CancelSubscription(gomock.Any(), subscription.ID, gomock.Any()).
			Return(nil)

		err := processor.SubscriptionDeleted(ctx, subscription)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("service error", func(t *testing.T) {
		subscription := stripe.Subscription{
			ID:     "sub_test123",
			Status: stripe.SubscriptionStatusCanceled,
		}

		mockSubscriptionService.EXPECT().
			CancelSubscription(gomock.Any(), subscription.ID, gomock.Any()).
			Return(errors.New("service error"))

		err := processor.SubscriptionDeleted(ctx, subscription)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestHandleWebhook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockStore := NewMockBillingStore(ctrl)
	mockProductService := NewMockProductService(ctrl)
	mockSubscriptionService := NewMockSubscriptionService(ctrl)
	mockEmailService := NewMockEmailService(ctrl)
	logger := observability.NewLogger()

	processor := New(
		"test_stripe_key",
		"test_webhook_secret",
		"http://localhost:3000",
		mockStore,
		mockProductService,
		mockSubscriptionService,
		mockEmailService,
		logger,
	)

	ctx := context.Background()

	t.Run("unhandled event type", func(t *testing.T) {
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "unknown.event",
		}

		err := processor.HandleWebhook(ctx, event)

		if err != nil {
			t.Errorf("expected no error for unhandled event, got %v", err)
		}
	})

	t.Run("product.created event", func(t *testing.T) {
		productJSON := []byte(`{"id": "prod_test123", "name": "Test Product", "description": "A test product"}`)
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "product.created",
			Data: &stripe.EventData{
				Raw: productJSON,
			},
		}

		mockProductService.EXPECT().
			CreateProduct(gomock.Any(), gomock.Any()).
			Return(nil)

		err := processor.HandleWebhook(ctx, event)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("product.created event with invalid JSON", func(t *testing.T) {
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "product.created",
			Data: &stripe.EventData{
				Raw: []byte(`invalid json`),
			},
		}

		err := processor.HandleWebhook(ctx, event)

		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})

	t.Run("price.created event", func(t *testing.T) {
		priceJSON := []byte(`{"id": "price_test123", "nickname": "Monthly Plan", "product": "prod_test123"}`)
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "price.created",
			Data: &stripe.EventData{
				Raw: priceJSON,
			},
		}

		mockProductService.EXPECT().
			CreatePrice(gomock.Any(), gomock.Any()).
			Return(nil)

		err := processor.HandleWebhook(ctx, event)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("price.updated event", func(t *testing.T) {
		priceJSON := []byte(`{"id": "price_test123", "nickname": "Updated Monthly Plan", "product": "prod_test123"}`)
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "price.updated",
			Data: &stripe.EventData{
				Raw: priceJSON,
			},
		}

		mockProductService.EXPECT().
			UpdatePrice(gomock.Any(), gomock.Any()).
			Return(nil)

		err := processor.HandleWebhook(ctx, event)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("price.deleted event", func(t *testing.T) {
		priceJSON := []byte(`{"id": "price_test123"}`)
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "price.deleted",
			Data: &stripe.EventData{
				Raw: priceJSON,
			},
		}

		mockProductService.EXPECT().
			DeletePrice(gomock.Any(), gomock.Any()).
			Return(nil)

		err := processor.HandleWebhook(ctx, event)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("customer.subscription.deleted event", func(t *testing.T) {
		subscriptionJSON := []byte(`{"id": "sub_test123", "status": "canceled"}`)
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "customer.subscription.deleted",
			Data: &stripe.EventData{
				Raw: subscriptionJSON,
			},
		}

		mockSubscriptionService.EXPECT().
			CancelSubscription(gomock.Any(), "sub_test123", gomock.Any()).
			Return(nil)

		err := processor.HandleWebhook(ctx, event)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("customer.subscription.updated event with previous status incomplete", func(t *testing.T) {
		subscriptionJSON := []byte(`{"id": "sub_test123", "status": "active", "items": {"data": [{"price": {"id": "price_test123"}}]}, "current_period_start": 1609459200, "current_period_end": 1612137600, "customer": "cus_test123"}`)
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "customer.subscription.updated",
			Data: &stripe.EventData{
				Raw: subscriptionJSON,
				PreviousAttributes: map[string]interface{}{
					"status": "incomplete",
				},
			},
		}

		mockSubscriptionService.EXPECT().
			CreateSubscription(gomock.Any(), gomock.Any()).
			Return(nil)

		err := processor.HandleWebhook(ctx, event)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("customer.subscription.updated event with previous status active", func(t *testing.T) {
		subscriptionJSON := []byte(`{"id": "sub_test123", "status": "active", "items": {"data": [{"price": {"id": "price_test123"}}]}, "current_period_end": 1612137600}`)
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "customer.subscription.updated",
			Data: &stripe.EventData{
				Raw: subscriptionJSON,
				PreviousAttributes: map[string]interface{}{
					"status": "active",
				},
			},
		}

		mockSubscriptionService.EXPECT().
			UpdateSubscription(gomock.Any(), gomock.Any()).
			Return(nil)

		err := processor.HandleWebhook(ctx, event)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("price.created event with service error", func(t *testing.T) {
		priceJSON := []byte(`{"id": "price_test123", "nickname": "Monthly Plan", "product": "prod_test123"}`)
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "price.created",
			Data: &stripe.EventData{
				Raw: priceJSON,
			},
		}

		mockProductService.EXPECT().
			CreatePrice(gomock.Any(), gomock.Any()).
			Return(errors.New("service error"))

		err := processor.HandleWebhook(ctx, event)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("price.updated event with service error", func(t *testing.T) {
		priceJSON := []byte(`{"id": "price_test123", "nickname": "Updated Plan", "product": "prod_test123"}`)
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "price.updated",
			Data: &stripe.EventData{
				Raw: priceJSON,
			},
		}

		mockProductService.EXPECT().
			UpdatePrice(gomock.Any(), gomock.Any()).
			Return(errors.New("service error"))

		err := processor.HandleWebhook(ctx, event)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("price.deleted event with service error", func(t *testing.T) {
		priceJSON := []byte(`{"id": "price_test123"}`)
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "price.deleted",
			Data: &stripe.EventData{
				Raw: priceJSON,
			},
		}

		mockProductService.EXPECT().
			DeletePrice(gomock.Any(), gomock.Any()).
			Return(errors.New("service error"))

		err := processor.HandleWebhook(ctx, event)

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("customer.subscription.deleted event with invalid JSON", func(t *testing.T) {
		event := stripe.Event{
			ID:   "evt_test123",
			Type: "customer.subscription.deleted",
			Data: &stripe.EventData{
				Raw: []byte(`invalid json`),
			},
		}

		err := processor.HandleWebhook(ctx, event)

		if err == nil {
			t.Error("expected error for invalid JSON, got nil")
		}
	})
}
