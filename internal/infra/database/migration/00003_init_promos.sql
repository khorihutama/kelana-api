-- +goose Up
CREATE TABLE promo_codes (
    id uuid PRIMARY KEY,
    code citext NOT NULL UNIQUE,
    discount_type text NOT NULL CHECK (discount_type IN ('PERCENT', 'FIXED')),
    discount_value int NOT NULL CHECK (discount_value > 0),
    min_transaction_idr bigint NOT NULL DEFAULT 0 CHECK (min_transaction_idr >= 0),
    quota int CHECK (quota IS NULL OR quota > 0),
    used_count int NOT NULL DEFAULT 0 CHECK (used_count >= 0),
    valid_from timestamptz NOT NULL,
    valid_to timestamptz NOT NULL,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT promo_codes_window_check CHECK (valid_to > valid_from),
    CONSTRAINT promo_codes_quota_limit_check CHECK (quota IS NULL OR used_count <= quota)
);

CREATE INDEX promo_codes_active_window_idx ON promo_codes (active, valid_from, valid_to);

-- +goose Down
DROP TABLE promo_codes;