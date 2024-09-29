package handler

import (
	"base-server/internal/money/billing/processor"
	"base-server/internal/observability"
)

type Handler struct {
	processor processor.BillingProcessor
	logger    *observability.Logger
}

func New(processor processor.BillingProcessor, logger *observability.Logger) Handler {
	return Handler{processor: processor, logger: logger}
}
