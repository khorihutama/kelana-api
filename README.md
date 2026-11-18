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

## Endpoint fase 1

| Method | Path | Auth |
|---|---|---|
| GET | `/api/v1/healthz` | Publik |
| GET | `/api/v1/readyz` | Publik |
| POST | `/api/v1/auth/register` | Publik |
| POST | `/api/v1/auth/verify-email` | Publik |
| POST | `/api/v1/auth/resend-verification` | Publik |
| POST | `/api/v1/auth/login` | Publik |
| POST | `/api/v1/auth/refresh` | Cookie refresh |
| POST | `/api/v1/auth/logout` | Bearer |
| GET | `/api/v1/auth/me` | Bearer |
| GET | `/api/v1/trips?page=1&limit=10` | Publik |

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

Fase 0-1 selesai. Fase berikutnya (discovery filter, seat lock Redis, dokumen, payment, my trips) belum diimplementasikan.

## Lisensi

Private project. Belum ada lisensi publik.