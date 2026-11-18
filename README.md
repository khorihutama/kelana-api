# HSI GO API

Backend Go untuk HSI GO, aplikasi travel dan itinerary. Fase 0-1 berisi walking skeleton: autentikasi dengan verifikasi email OTP, health check, dan daftar trip dengan pagination.

## Stack

Go 1.24, Gin, GORM, PostgreSQL 17, Redis 7, goose (migrasi SQL), JWT access token + refresh token HttpOnly.

## Prasyarat

- Go 1.24+
- Docker Compose v2

## Menjalankan

```bash
cp .env.example .env          # sesuaikan JWT_SECRET (minimal 32 karakter)
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
| `SEAT_SWEEP_INTERVAL_SECONDS` | Interval sweeper, default `30` detik |

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

## Status

Fase 0-3 selesai: bootstrap, auth OTP, discovery, booking dengan seat lock Redis dan promo. Fase berikutnya (dokumen, payment, my trips) belum diimplementasikan.

## Lisensi

Private project. Belum ada lisensi publik.