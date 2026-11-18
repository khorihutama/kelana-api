-- +goose Up
CREATE INDEX trips_destination_trgm_idx ON trips USING gin (destination_city gin_trgm_ops);
CREATE INDEX trips_departure_trgm_idx ON trips USING gin (departure_city gin_trgm_ops);
CREATE INDEX trips_destination_lower_idx ON trips (lower(destination_city));
CREATE INDEX trips_departure_lower_idx ON trips (lower(departure_city));
CREATE INDEX trips_base_price_idx ON trips (base_price_idr);
CREATE INDEX trips_rating_idx ON trips (rating_avg DESC, rating_count DESC);
CREATE INDEX trips_created_idx ON trips (created_at DESC);
CREATE INDEX trips_category_price_idx ON trips (category, base_price_idr);
CREATE UNIQUE INDEX trip_facilities_trip_name_uq ON trip_facilities (trip_id, name);

-- +goose Down
DROP INDEX trip_facilities_trip_name_uq;
DROP INDEX trips_category_price_idx;
DROP INDEX trips_created_idx;
DROP INDEX trips_rating_idx;
DROP INDEX trips_base_price_idx;
DROP INDEX trips_departure_lower_idx;
DROP INDEX trips_destination_lower_idx;
DROP INDEX trips_departure_trgm_idx;
DROP INDEX trips_destination_trgm_idx;