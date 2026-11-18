# HSI GO API

Backend Go untuk HSI GO, aplikasi travel dan itinerary. Fase 0-4 berisi autentikasi OTP, discovery trip, booking dengan seat lock Redis, dan dokumen terenkripsi.

## Stack

Go 1.24, Gin, GORM, PostgreSQL 17, Redis 7, goose (migrasi SQL), JWT access token + refresh token HttpOnly.

## Prasyarat

- Go 1.24+
- Docker Compose v2

## Menjalankan

```bash
cp .env.example .env          # sesuaikan JWT_SECRET (minimal 32 karakter)
# Isi DOCUMENT_ENC_KEY dengan base64 dari 32 byte acak: openssl rand -base64 32
# Jika port 5432 sudah dipakai stack lain, ubah DB_PORT=5433 di .env.
# Compose memetakan DB_PORT host ke 5432 di container.
make docker-up                # postgres + redis
make migrate-up
make seed
make run
```

Server berjalan di `http://localhost:8080`.

## Environment

| Key | Keterangan |
|---|---|
| `APP_ENV` | `development` atau `production` |
| `REST_PORT` | Port HTTP, default `8080` |
| `DB_HOST`, `DB_PORT`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD` | Koneksi PostgreSQL |
| `DB_SCHEMA` | Schema, default `public` |
| `REDIS_ADDR` | Alamat Redis, default `localhost:6379` |
| `JWT_SECRET` | Minimal 32 karakter; dipakai access token dan HMAC OTP |
| `REFRESH_COOKIE_NAME` | Nama cookie refresh, default `hsi_refresh` |
| `CORS_ORIGINS` | Origin diizinkan, pisahkan dengan koma |
| `MAIL_DRIVER` | Fase 1 hanya `mock` (kode OTP ditulis ke log) |
| `SEED_ENABLED` | `true` untuk mengizinkan `make seed` |
| `SEAT_LOCK_TTL_SECONDS` | Masa hold kursi, default `900` detik |
| `SEAT_HOLD_SAFETY_TTL_SECONDS` | TTL cadangan key hold, default `1800`; wajib > `SEAT_LOCK_TTL_SECONDS` |
| `SEAT_SWEEP_INTERVAL_SECONDS` | Interval sweeper, default `30` detik |
| `STORAGE_DRIVER` | Fase 4 hanya `local` |
| `UPLOADS_DIR` | Root data upload; file dokumen masuk ke `<dir>/documents` dan tidak disajikan statis |
| `DOCUMENT_ENC_KEY` | Base64 32 byte untuk AES-256-GCM; wajib, jangan commit |
| `DOCUMENT_URL_TTL_SECONDS` | Masa berlaku signed URL, default `300` detik |

## Endpoint

Health tersedia di root (`/healthz`, `/readyz`) dan di `/api/v1`.

| Method | Path | Auth |
|---|---|---|
| GET | `/healthz`, `/readyz` | Publik |
| GET | `/api/v1/healthz`, `/api/v1/readyz` | Publik |
| POST | `/api/v1/auth/register` | Publik |
| POST | `/api/v1/auth/verify-email` | Publik |
| POST | `/api/v1/auth/resend-verification` | Publik |
| POST | `/api/v1/auth/login` | Publik |
| POST | `/api/v1/auth/refresh` | Cookie refresh |
| POST | `/api/v1/auth/logout` | Bearer |
| GET | `/api/v1/auth/me` | Bearer |
| GET | `/api/v1/trips` | Publik |
| GET | `/api/v1/trips/{id-or-slug}` | Publik |
| GET | `/api/v1/trips/{id-or-slug}/schedules` | Publik |
| GET | `/api/v1/destinations` | Publik |
| GET | `/api/v1/categories` | Publik |
| POST | `/api/v1/reservations/lock` | Bearer |
| GET | `/api/v1/reservations/{id}` | Bearer (pemilik) |
| POST | `/api/v1/reservations/{id}/cancel` | Bearer (pemilik) |
| POST | `/api/v1/promos/validate` | Bearer |
| POST | `/api/v1/documents` | Bearer (multipart: `file`, `type`, `participant_id`) |
| GET | `/api/v1/documents/{id}/url` | Bearer (pemilik) |
| GET | `/api/v1/documents/{id}/content?exp=&sig=` | Signed URL |
| DELETE | `/api/v1/documents/{id}` | Bearer (pemilik) |
| GET | `/api/v1/reservations/{id}/documents/completeness` | Bearer (pemilik) |

Filter `GET /api/v1/trips`: `q`, `category` (OPEN/PRIVATE/EDU), `destination`, `departure`, `date_from`, `date_to`, `price_min_idr`, `price_max_idr`, `sort` (newest/price_asc/price_desc/rating_desc/departure_asc), `page`, `limit` (maks 100). Filter tidak valid menghasilkan 422 dengan envelope standar.

Register membuat akun berstatus `PENDING_VERIFICATION` dan mengirim OTP 6 digit. Di fase 1 mail driver `mock` menulis kode ke log server. Login sebelum verifikasi ditolak dengan `EMAIL_NOT_VERIFIED`.

## Perintah

```bash
make test         # unit test
make vet
make build
make migrate-up
make migrate-status
make seed
make run
```

## Dokumen

`POST /api/v1/documents` menerima `file` (JPEG/PNG/PDF asli, maks 3 MB), `type` (`KTP`, `BUKU_NIKAH`, `KK`, `PASSPORT`, `OTHER`), dan `participant_id` milik reservasi yang masih `PENDING_PAYMENT`. File dienkripsi AES-256-GCM sebelum ditulis ke disk; nonce 12 byte disimpan di awal file, checksum SHA-256 dari plaintext disimpan di DB. Tidak ada static route ke folder upload.

`GET /api/v1/documents/{id}/url` mengembalikan signed URL 5 menit. Tanda tangan HMAC-SHA256 mengikat document id dan waktu kedaluwarsa; URL kedaluwarsa ditolak `410 URL_EXPIRED`. Setelah reservasi `PAID`, dokumen tidak bisa dihapus (`409 ALREADY_PAID`).

## Status

Fase 0-4 selesai: bootstrap, auth OTP, discovery, booking dengan seat lock Redis dan promo, serta dokumen terenkripsi. Fase berikutnya (payment, my trips) belum diimplementasikan.

## Lisensi

Private project. Belum ada lisensi publik.