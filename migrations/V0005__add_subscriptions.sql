CREATE TABLE subscriptions
(
    id                UUID PRIMARY KEY,
    user_id           UUID REFERENCES Users (id),
    price_id          UUID REFERENCES prices (id),
    stripe_id         VARCHAR(255) NOT NULL,
    status            VARCHAR(50) CHECK (status IN ('active', 'inactive', 'canceled', 'past_due', 'trialing')),
    start_date        TIMESTAMP,
    end_date          TIMESTAMP,
    next_billing_date TIMESTAMP,
    created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at        TIMESTAMP DEFAULT NULL
);