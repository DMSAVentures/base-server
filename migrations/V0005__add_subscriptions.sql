CREATE TABLE subscriptions
(
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID REFERENCES Users (id),
    price_id          UUID REFERENCES prices (id),
    stripe_id         VARCHAR(255) NOT NULL,
    status            VARCHAR(50) CHECK (status IN ('active', 'inactive', 'canceled', 'past_due', 'trialing')),
    start_date        TIMESTAMPTZ DEFAULT NULL,
    end_date          TIMESTAMPTZ DEFAULT NULL,
    next_billing_date TIMESTAMPTZ DEFAULT NULL,
    created_at        TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at        TIMESTAMPTZ DEFAULT NULL
);