-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE trips (
    id uuid PRIMARY KEY,
    slug text NOT NULL UNIQUE,
    title text NOT NULL,
    description text NOT NULL,
    category text NOT NULL CHECK (category IN ('OPEN', 'PRIVATE', 'EDU')),
    departure_city text NOT NULL,
    destination_city text NOT NULL,
    duration_days int NOT NULL CHECK (duration_days > 0),
    base_price_idr bigint NOT NULL CHECK (base_price_idr >= 0),
    currency char(3) NOT NULL DEFAULT 'IDR',
    status text NOT NULL DEFAULT 'DRAFT' CHECK (status IN ('DRAFT', 'PUBLISHED', 'ARCHIVED')),
    hero_image_url text,
    wa_group_url text,
    required_documents text[] NOT NULL DEFAULT '{KTP}',
    rating_avg numeric(3,2) NOT NULL DEFAULT 0 CHECK (rating_avg >= 0 AND rating_avg <= 5),
    rating_count int NOT NULL DEFAULT 0 CHECK (rating_count >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX trips_status_category_destination_idx ON trips (status, category, destination_city);
CREATE INDEX trips_status_departure_idx ON trips (status, departure_city);
CREATE INDEX trips_title_trgm_idx ON trips USING gin (title gin_trgm_ops);

CREATE TABLE trip_schedules (
    id uuid PRIMARY KEY,
    trip_id uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    start_date date NOT NULL,
    end_date date NOT NULL,
    departure_point text NOT NULL,
    quota_total int NOT NULL CHECK (quota_total > 0),
    seats_sold int NOT NULL DEFAULT 0 CHECK (seats_sold >= 0),
    price_idr bigint NOT NULL CHECK (price_idr >= 0),
    currency char(3) NOT NULL DEFAULT 'IDR',
    status text NOT NULL DEFAULT 'OPEN' CHECK (status IN ('OPEN', 'CLOSED', 'CANCELLED')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT trip_schedules_dates_check CHECK (end_date >= start_date),
    CONSTRAINT trip_schedules_quota_check CHECK (seats_sold <= quota_total),
    CONSTRAINT trip_schedules_unique UNIQUE (trip_id, start_date)
);

CREATE INDEX trip_schedules_trip_date_idx ON trip_schedules (trip_id, start_date);
CREATE INDEX trip_schedules_status_date_idx ON trip_schedules (status, start_date);

CREATE TABLE trip_itineraries (
    id uuid PRIMARY KEY,
    trip_id uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    day_number int NOT NULL CHECK (day_number > 0),
    title text NOT NULL,
    description text NOT NULL,
    sort_order int NOT NULL DEFAULT 0,
    UNIQUE (trip_id, day_number)
);

CREATE TABLE trip_facilities (
    id uuid PRIMARY KEY,
    trip_id uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    name text NOT NULL,
    type text NOT NULL CHECK (type IN ('INCLUDED', 'EXCLUDED')),
    sort_order int NOT NULL DEFAULT 0
);

CREATE TABLE hotels (
    id uuid PRIMARY KEY,
    name text NOT NULL,
    address text NOT NULL,
    facilities text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX hotels_name_idx ON hotels (name);

CREATE TABLE trip_hotels (
    id uuid PRIMARY KEY,
    trip_id uuid NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    hotel_id uuid NOT NULL REFERENCES hotels(id),
    extra_cost_idr bigint NOT NULL DEFAULT 0 CHECK (extra_cost_idr >= 0),
    sort_order int NOT NULL DEFAULT 0,
    UNIQUE (trip_id, hotel_id)
);

CREATE INDEX trip_hotels_trip_sort_idx ON trip_hotels (trip_id, sort_order);

-- +goose Down
DROP TABLE trip_hotels;
DROP TABLE hotels;
DROP TABLE trip_facilities;
DROP TABLE trip_itineraries;
DROP TABLE trip_schedules;
DROP TABLE trips;