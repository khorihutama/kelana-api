# HSI GO API

Backend Go untuk HSI GO, aplikasi travel dan itinerary. Fase 0-6 berisi autentikasi OTP, discovery trip, booking dengan seat lock Redis, dokumen terenkripsi, payment mock, dan my trips dengan expiry job.

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
| `PAYMENT_PROVIDER` | Fase 5 hanya `mock`; adapter Midtrans ada tetapi belum diaktifkan |
| `MOCK_WEBHOOK_SECRET` | Secret HMAC webhook mock, minimal 16 karakter |
| `PAYMENT_TTL_SECONDS` | Masa bayar terpisah dari lock, default `86400` detik (24 jam) |
| `PAYMENT_EXPIRY_INTERVAL_SECONDS` | Interval job expiry payment, default `30` detik |
| `WA_GROUP_URL` | Fallback link grup WhatsApp; URL per trip (`trips.wa_group_url`) lebih diutamakan |

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
| POST | `/api/v1/reservations/{id}/payments` | Bearer (pemilik) |
| GET | `/api/v1/payments/{id}` | Bearer (pemilik) |
| POST | `/api/v1/payments/webhook/{provider}` | Publik + HMAC |
| GET | `/api/v1/mytrips` | Bearer (hanya milik user) |
| GET | `/api/v1/mytrips/{id}` | Bearer (pemilik) |
| POST | `/api/v1/mytrips/{id}/cancel` | Bearer (pemilik) |

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

## Pembayaran mock

`POST /api/v1/reservations/{id}/payments` hanya untuk pemilik, reservasi `PENDING_PAYMENT` yang lock-nya belum lewat, dan seluruh dokumen wajib sudah lengkap. Body: `{"method":"VA_BCA"}` (juga `VA_BSI`, `VA_MANDIRI`, `QRIS`, `CARD`). Respons memuat `amount_idr`, `va_number`/`qr_string`, dan `expires_at` 24 jam.

Webhook mock tidak memakai JWT. Kirim raw body:

```json
{"event_id":"evt-1","payment_id":"<uuid>","status":"PAID","amount_idr":5900000}
```

Header `X-Mock-Signature` berisi hex HMAC-SHA256 raw body dengan `MOCK_WEBHOOK_SECRET`. Signature diverifikasi sebelum DB disentuh. Event id unik membuat retry dan 20 request paralel hanya berefek sekali. Webhook valid menandai payment dan reservasi `PAID`, menambah `seats_sold`, dan menghapus hold Redis tanpa mengembalikan seat.

`GET /api/v1/payments/{id}` hanya membaca status; tidak pernah mengubahnya. Refund mengembalikan `ErrNotSupported`; belum ada endpoint refund. Adapter Midtrans ada di `internal/infra/payment/midtrans.go` dengan skema signature SHA512 dan mapping status, tetapi tidak didaftarkan di DI.

## My Trips

`GET /api/v1/mytrips?tab=BELUM_BAYAR|SIAP_BERANGKAT|SELESAI|RIWAYAT&page=1&limit=10` hanya mengembalikan reservasi milik user. Tanpa `tab`, semua status dikembalikan. Pemetaan:

| Tab | Status |
|---|---|
| `BELUM_BAYAR` | `PENDING_PAYMENT` |
| `SIAP_BERANGKAT` | `PAID` dengan `end_date >= hari ini` (WIB) |
| `SELESAI` | `PAID` dengan `end_date < hari ini`, atau `COMPLETED` |
| `RIWAYAT` | `CANCELLED`, `EXPIRED` |

`GET /api/v1/mytrips/{id}` memuat peserta dan metadata dokumen tanpa signed URL. `join_wa_url` hanya ada jika reservasi `PAID`; status lain tidak memuat field itu sama sekali. `POST /api/v1/mytrips/{id}/cancel` boleh saat `PENDING_PAYMENT`; setelah `PAID` ditolak `409 ALREADY_PAID`.

Job expiry berjalan di background:
- Reservasi `PENDING_PAYMENT` lewat `lock_expires_at` -> `EXPIRED` dan seat dikembalikan; job idempoten dan memakai query DB sebagai fallback bila zset Redis hilang.
- Payment `PENDING` lewat `expires_at` -> `EXPIRED`; reservasi yang masih `PENDING_PAYMENT` ikut `EXPIRED`. Payment/reservasi `PAID` tidak pernah disentuh, jadi seat tidak kembali.

## Status

Fase 0-6 selesai: bootstrap, auth OTP, discovery, booking dengan seat lock Redis dan promo, dokumen terenkripsi, payment mock + webhook idempoten, serta my trips + expiry job. Fase berikutnya (hardening) belum diimplementasikan.

## Lisensi

Private project. Belum ada lisensi publik.