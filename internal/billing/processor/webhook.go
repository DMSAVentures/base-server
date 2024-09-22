package processor

import (
	"base-server/internal/observability"
	"context"
	"encoding/json"

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
	case "payment_intent.succeeded":
		var paymentIntent stripe.PaymentIntent
		err := json.Unmarshal(event.Data.Raw, &paymentIntent)
		if err != nil {
			p.logger.Error(ctx, "failed to unmarshal payment intent", err)
			return err
		}
		p.PaymentIntentSucceeded(ctx, paymentIntent)
	case "product.created":
		var product stripe.Product
		err := json.Unmarshal(event.Data.Raw, &product)
		if err != nil {
			p.logger.Error(ctx, "failed to unmarshal product", err)
			return err
		}
		p.ProductCreated(ctx, product)
	case "price.created":
		var price stripe.Price
		err := json.Unmarshal(event.Data.Raw, &price)
		if err != nil {
			p.logger.Error(ctx, "failed to unmarshal plan", err)
			return err
		}
		p.PriceCreated(ctx, price)
	case "price.updated":
		var price stripe.Price
		err := json.Unmarshal(event.Data.Raw, &price)
		if err != nil {
			p.logger.Error(ctx, "failed to unmarshal plan", err)
			return err
		}
		p.PriceUpdated(ctx, price)
	case "price.deleted":
		var price stripe.Price
		err := json.Unmarshal(event.Data.Raw, &price)
		if err != nil {
			p.logger.Error(ctx, "failed to unmarshal plan", err)
			return err
		}
		p.PriceDeleted(ctx, price)
	}

	return nil
}
