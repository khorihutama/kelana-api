-- +goose Up
CREATE TABLE payments (
    id uuid PRIMARY KEY,
    reservation_id uuid NOT NULL UNIQUE REFERENCES reservations(id) ON DELETE CASCADE,
    provider text NOT NULL CHECK (provider IN ('MOCK', 'MIDTRANS', 'XENDIT')),
    provider_ref text NOT NULL,
    method text NOT NULL CHECK (method IN ('VA_BCA', 'VA_BSI', 'VA_MANDIRI', 'QRIS', 'CARD')),
    amount_idr bigint NOT NULL CHECK (amount_idr >= 0),
    currency char(3) NOT NULL DEFAULT 'IDR',
    status text NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING', 'PAID', 'EXPIRED', 'FAILED', 'REFUNDED')),
    va_number text,
    qr_string text,
    expires_at timestamptz NOT NULL,
    paid_at timestamptz,
    raw_response jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_ref)
);

CREATE INDEX payments_status_expires_idx ON payments (status, expires_at);

CREATE TABLE payment_events (
    id uuid PRIMARY KEY,
    payment_id uuid NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    provider text NOT NULL,
    provider_event_id text NOT NULL,
    event_type text NOT NULL,
    signature_valid boolean NOT NULL DEFAULT false,
    payload jsonb NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_event_id)
);

CREATE INDEX payment_events_payment_idx ON payment_events (payment_id);

-- +goose Down
DROP TABLE payment_events;
DROP TABLE payments;