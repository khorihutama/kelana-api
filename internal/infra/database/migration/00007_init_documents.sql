-- +goose Up
CREATE TABLE documents (
    id uuid PRIMARY KEY,
    reservation_id uuid NOT NULL REFERENCES reservations(id) ON DELETE CASCADE,
    participant_id uuid NOT NULL REFERENCES participants(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id),
    type text NOT NULL CHECK (type IN ('KTP', 'BUKU_NIKAH', 'KK', 'PASSPORT', 'OTHER')),
    object_key text NOT NULL UNIQUE,
    mime_type text NOT NULL CHECK (mime_type IN ('image/jpeg', 'image/png', 'application/pdf')),
    size_bytes bigint NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 3145728),
    checksum_sha256 text NOT NULL,
    status text NOT NULL DEFAULT 'UPLOADED' CHECK (status IN ('UPLOADED', 'VERIFIED', 'REJECTED')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX documents_reservation_type_idx ON documents (reservation_id, type);
CREATE INDEX documents_participant_idx ON documents (participant_id);
CREATE INDEX documents_user_idx ON documents (user_id);

-- +goose Down
DROP TABLE documents;