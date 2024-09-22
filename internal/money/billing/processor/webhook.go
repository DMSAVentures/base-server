package processor

import (
	"base-server/internal/observability"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stripe/stripe-go/v79"
	"github.com/stripe/stripe-go/v79/tax/transaction"
)

// Invoke this method in your webhook handler when `payment_intent.succeeded` webhook is received
func (p *BillingProcessor) PaymentIntentSucceeded(ctx context.Context, paymentIntent stripe.PaymentIntent) {
	// Create a Tax Transaction for the successful payment
	params := &stripe.TaxTransactionCreateFromCalculationParams{
		Calculation: stripe.String(paymentIntent.Metadata["tax_calculation"]),
		Reference:   stripe.String("123"),
	}
	params.AddExpand("line_items")

	txn, err := transaction.CreateFromCalculation(params)
	if err != nil {
		p.logger.Error(ctx, "failed to create tax transaction", err)
		return
	}

	ctx = observability.WithFields(ctx, observability.Field{"tax_transaction_id", txn.ID})
	p.logger.Info(ctx, "Tax transaction created")

	//// 2. Update order status to 'Paid' in the system (pseudo-code)
	//orderID := paymentIntent.Metadata["order_id"]
	//err = p.orderService.MarkOrderAsPaid(ctx, orderID, txnID)
	//if err != nil {
	//	p.logger.Error(ctx, "failed to mark order as paid", err)
	//	return
	//}
	//
	//p.logger.Info(ctx, "Order marked as paid")
	//
	//// 3. Send confirmation email or notification to the customer (pseudo-code)
	//customerEmail := paymentIntent.ReceiptEmail // From the payment intent
	//err = p.emailService.SendPaymentConfirmation(ctx, customerEmail, orderID)
	//if err != nil {
	//	p.logger.Error(ctx, "failed to send confirmation email", err)
	//	return
	//}
	//
	//p.logger.Info(ctx, "Payment confirmation sent to", customerEmail)
	//
	//// 4. Fulfill the order (pseudo-code)
	//err = p.fulfillmentService.FulfillOrder(ctx, orderID)
	//if err != nil {
	//	p.logger.Error(ctx, "failed to fulfill order", err)
	//	return
	//}

	//p.logger.Info(ctx, "Order fulfilled", orderID)
}

// Invoke this method in your webhook handler when `customer.subscription.updated` webhook is received
func (p *BillingProcessor) SubscriptionUpdated(ctx context.Context, subscription stripe.Subscription) {
	err := p.subscriptionService.UpdateSubscription(ctx, subscription)
	if err != nil {
		p.logger.Error(ctx, "failed to update subscription", err)
		return
	}
	p.logger.Info(ctx, "Subscription updated")
}

// Invoke this method in your webhook handler when `customer.subscription.updated` webhook is received
func (p *BillingProcessor) SubscriptionCreated(ctx context.Context, subscription stripe.Subscription) {
	err := p.subscriptionService.CreateSubscription(ctx, subscription)
	if err != nil {
		p.logger.Error(ctx, "failed to create subscription", err)
		return
	} else {
		p.logger.Info(ctx, "Subscription created")
	}
}

// Invoke this method in your webhook handler when `invoice.payment_failed` webhook is received
func (p *BillingProcessor) InvoicePaymentFailed(ctx context.Context, invoice stripe.Invoice) {
	// 1. Get the user by the customer ID
	// 2. Get the user's email
	// 3. Send an email to the user
	err := p.subscriptionService.CancelSubscription(ctx, invoice.Subscription.ID)
	if err != nil {
		p.logger.Error(ctx, "failed to cancel subscription", err)
		return
	}
}

func (p *BillingProcessor) InvoicePaymentPaid(ctx context.Context, invoice stripe.Invoice) {
	// 1. Get the user by the customer ID
	// 2. Get the user's email
	// 3. Send an email to the user
	// 4. Update next billing date on subscription row
	//err := p.subscriptionService.UpdateSubscription(ctx, invoice.Subscription.ID)
	//if err != nil {
	//	p.logger.Error(ctx, "failed to cancel subscription", err)
	//	return
	//}
}

//// Invoke this method in your webhook handler when `invoice.payment_succeeded` webhook is received
//func (p *BillingProcessor) InvoicePaymentSucceeded(ctx context.Context, invoice stripe.Invoice) {
//	// 1. Get the user by the customer ID
//	// 2. Get the user's email
//	// 3. Send an email to the user
//	err := p.subscriptionService.UpdateSubscription(ctx, invoice.Subscription.ID)
//	if err != nil {
//		p.logger.Error(ctx, "failed to cancel subscription", err)
//		return
//	}
//}

func (p *BillingProcessor) ProductCreated(ctx context.Context, productCreated stripe.Product) {
	err := p.productService.CreateProduct(ctx, productCreated)
	if err != nil {
		p.logger.Error(ctx, "failed to handle product creation webhook event", err)
		return
	}
}

func (p *BillingProcessor) PriceCreated(ctx context.Context, priceCreated stripe.Price) {
	err := p.productService.CreatePrice(ctx, priceCreated)
	if err != nil {
		p.logger.Error(ctx, "failed to handle plan creation webhook event", err)
		return
	}
}

func (p *BillingProcessor) PriceUpdated(ctx context.Context, priceUpdated stripe.Price) {
	err := p.productService.UpdatePrice(ctx, priceUpdated)
	if err != nil {
		p.logger.Error(ctx, "failed to handle plan update webhook event", err)
		return
	}
}

func (p *BillingProcessor) PriceDeleted(ctx context.Context, priceDeleted stripe.Price) {
	err := p.productService.DeletePrice(ctx, priceDeleted)
	if err != nil {
		p.logger.Error(ctx, "failed to handle plan deletion webhook event", err)
		return
	}
}

func (p *BillingProcessor) HandleWebhook(ctx context.Context, event stripe.Event) error {
	// Handle the event
	switch event.Type {
	case "product.created":
		var product stripe.Product
		err := json.Unmarshal(event.Data.Raw, &product)
		if err != nil {
			p.logger.Error(ctx, "failed to unmarshal product", err)
			return err
		}
		p.ProductCreated(ctx, product)

	case "price.created", "price.updated", "price.deleted":
		var price stripe.Price
		err := json.Unmarshal(event.Data.Raw, &price)
		if err != nil {
			p.logger.Error(ctx, "failed to unmarshal price", err)
			return err
		}
		switch event.Type {
		case "price.created":
			p.PriceCreated(ctx, price)
		case "price.updated":
			p.PriceUpdated(ctx, price)
		case "price.deleted":
			p.PriceDeleted(ctx, price)
		}

	case "customer.subscription.updated":
		var subscription stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &subscription)
		if err != nil {
			p.logger.Error(ctx, "failed to unmarshal price", err)
			return err
		}

		// Check for previous subscription status
		previousStatus, ok := event.Data.PreviousAttributes["status"].(string)
		if !ok {
			err := errors.New("failed to retrieve previous subscription status")
			p.logger.Error(ctx, "subscription updated event missing previous status", err)
			//return err
		}

		// Subscription status handling
		if previousStatus == "incomplete" {
			p.logger.Info(ctx, "Subscription moved from incomplete to active")
			p.SubscriptionCreated(ctx, subscription)
		} else {
			p.logger.Info(ctx, "Subscription updated")
			p.SubscriptionUpdated(ctx, subscription)
		}

	case "invoice.payment_failed":
		var invoice stripe.Invoice
		err := json.Unmarshal(event.Data.Raw, &invoice)
		if err != nil {
			p.logger.Error(ctx, "failed to unmarshal price", err)
			return err
		}
		p.InvoicePaymentFailed(ctx, invoice)

	case "invoice.paid":
		var invoice stripe.Invoice
		err := json.Unmarshal(event.Data.Raw, &invoice)
		if err != nil {
			p.logger.Error(ctx, "failed to unmarshal price", err)
			return err
		}
		p.InvoicePaymentPaid(ctx, invoice)

	default:
		p.logger.Warn(ctx, fmt.Sprintf("Unhandled event type: %s", event.Type))
	}

	return nil
}
