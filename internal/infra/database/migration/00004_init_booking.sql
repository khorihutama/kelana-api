-- +goose Up
CREATE TABLE reservations (
    id uuid PRIMARY KEY,
    code text NOT NULL UNIQUE,
    user_id uuid NOT NULL REFERENCES users(id),
    trip_id uuid NOT NULL REFERENCES trips(id),
    schedule_id uuid NOT NULL REFERENCES trip_schedules(id),
    status text NOT NULL DEFAULT 'PENDING_PAYMENT' CHECK (status IN ('PENDING_PAYMENT', 'PAID', 'EXPIRED', 'CANCELLED', 'COMPLETED')),
    participants_count int NOT NULL CHECK (participants_count BETWEEN 1 AND 10),
    unit_price_idr bigint NOT NULL CHECK (unit_price_idr >= 0),
    total_amount_idr bigint NOT NULL CHECK (total_amount_idr >= 0),
    discount_idr bigint NOT NULL DEFAULT 0 CHECK (discount_idr >= 0),
    currency char(3) NOT NULL DEFAULT 'IDR',
    pickup_point text NOT NULL,
    notes text,
    hotel_id uuid REFERENCES trip_hotels(id),
    promo_code_id uuid REFERENCES promo_codes(id),
    lock_expires_at timestamptz NOT NULL,
    payment_expires_at timestamptz,
    paid_at timestamptz,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX reservations_user_status_created_idx ON reservations (user_id, status, created_at DESC);
CREATE INDEX reservations_schedule_status_idx ON reservations (schedule_id, status);
CREATE INDEX reservations_status_lock_expires_idx ON reservations (status, lock_expires_at);
CREATE UNIQUE INDEX reservations_active_unique ON reservations (user_id, schedule_id)
    WHERE status IN ('PENDING_PAYMENT', 'PAID');

CREATE TABLE participants (
    id uuid PRIMARY KEY,
    reservation_id uuid NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    full_name text NOT NULL,
    gender text NOT NULL CHECK (gender IN ('MALE', 'FEMALE')),
    email text NOT NULL,
    phone text NOT NULL,
    id_number_encrypted bytea,
    is_primary boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX participants_reservation_idx ON participants (reservation_id);

CREATE TABLE promo_redemptions (
    id uuid PRIMARY KEY,
    promo_code_id uuid NOT NULL REFERENCES promo_codes(id),
    reservation_id uuid NOT NULL UNIQUE REFERENCES reservations(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id),
    discount_idr bigint NOT NULL CHECK (discount_idr >= 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX promo_redemptions_promo_idx ON promo_redemptions (promo_code_id);

-- +goose Down
DROP TABLE promo_redemptions;
DROP TABLE participants;
DROP TABLE reservations;