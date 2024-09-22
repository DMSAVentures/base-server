-- Migration: Create features table (represents different features available in the system)
CREATE TABLE features
(
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPZ DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMPZ DEFAULT NULL,
    UNIQUE (name) -- Ensure feature names are unique
);

-- Migration: Create limits table (represents limits associated with specific features)
CREATE TABLE limits
(
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    feature_id  UUID REFERENCES features (id),
    limit_name  VARCHAR(255) NOT NULL,
    limit_value INTEGER      NOT NULL, -- Example: 10 API calls or 5GB storage
    created_at  TIMESTAMPZ DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMPZ DEFAULT NULL,
    UNIQUE (feature_id, limit_name)    -- Ensure limit names are unique per feature
);


-- Migration: Create plan_feature_limits table (links plans with features and limits)
CREATE TABLE plan_feature_limits
(
    plan_id    UUID REFERENCES prices (id),
    feature_id UUID REFERENCES features (id),
    limit_id   UUID REFERENCES limits (id),
    enabled    BOOLEAN   DEFAULT TRUE, -- Whether the feature is available in this plan
    created_at TIMESTAMPZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPZ DEFAULT NULL,
    PRIMARY KEY (plan_id, feature_id)

);