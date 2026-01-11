-- Database initialization script for the Scalable Coupon System

-- Coupons table
CREATE TABLE coupons (
    name VARCHAR(255) PRIMARY KEY,
    amount INTEGER NOT NULL CHECK (amount > 0),
    remaining_amount INTEGER NOT NULL CHECK (remaining_amount >= 0),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Claims table (separate, no embedding per architecture)
CREATE TABLE claims (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    coupon_name VARCHAR(255) NOT NULL REFERENCES coupons(name),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, coupon_name)
);

-- Index for efficient claim lookups by coupon
CREATE INDEX idx_claims_coupon_name ON claims(coupon_name);

-- Index for efficient claim lookups by user
CREATE INDEX idx_claims_user_id ON claims(user_id);
