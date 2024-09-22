-- Migration: Create products table
CREATE TABLE products
(
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(255) NOT NULL,
    description TEXT,
    stripe_id   VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMPTZ DEFAULT NULL
);

-- Migration: Create prices table (represents recurring, one-time, or lifetime price plans for products)
CREATE TABLE prices
(
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    product_id  UUID REFERENCES products (id),
    stripe_id   VARCHAR(255) NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMPTZ DEFAULT NULL
);
