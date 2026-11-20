# Arsitektur Backend Kelana

Dokumen ini menggambarkan arsitektur dan alur bisnis backend Kelana sesuai kode di repository. Nama route, key Redis, tabel, dan package diverifikasi langsung dari kode; sumbernya dicantumkan di bagian akhir.

## 1. Konteks sistem

```mermaid
flowchart LR
    Client[Client web atau mobile] --> API[API Go Gin]
    API --> PG[(PostgreSQL 17)]
    API --> Redis[(Redis 7)]
    API --> Storage[(Storage lokal terenkripsi)]
    API --> Mailer[Mock mailer OTP]
    API --> Provider[Mock payment provider]
    Provider -. adapter tidak aktif .-> Midtrans[Midtrans skeleton]
```

Klien frontend belum ada. API berjalan sebagai satu binary Go dan memakai PostgreSQL untuk data transaksi, Redis untuk seat hold dan rate limit, storage lokal untuk file dokumen terenkripsi, serta provider payment mock.

## 2. Diagram komponen

```mermaid
flowchart TB
    Client[Client] --> MW[Middleware chain]

    subgraph MWDetail[Middleware]
        RequestID[Request ID]
        Recovery[Recovery]
        Security[Security headers]
        CORS[CORS allowlist]
        BodyLimit[Body limit]
        Auth[Auth Bearer]
        RateLimit[Rate limit Redis]
        Logger[Request logger]
    end

    MW --> MWDetail
    MWDetail --> Handler[Handler per module]

    subgraph Modules[Module aplikasi]
        AuthM[Auth]
        TripM[Trip]
        BookingM[Booking]
        DocumentM[Document]
        PaymentM[Payment]
        MyTripM[MyTrip]
        HealthM[Health]
    end

    Handler --> AuthM
    Handler --> TripM
    Handler --> BookingM
    Handler --> DocumentM
    Handler --> PaymentM
    Handler --> MyTripM
    Handler --> HealthM

    AuthM --> Service[Service layer]
    TripM --> Service
    BookingM --> Service
    DocumentM --> Service
    PaymentM --> Service
    MyTripM --> Service
    HealthM --> Service

    Service --> Repo[Repository layer]
    Repo --> PG[(PostgreSQL)]
    Service --> Redis[(Redis)]
    Service --> Storage[(Storage lokal)]
    Service --> MockPay[Mock payment provider]
    Service --> MockMail[Mock mailer]
    Service --> Log[JSON slog]
```

Aturan dependensi: handler memanggil service, service memanggil repository atau port infra. Module `mytrip` memakai query read-only sendiri dan mendelegasikan cancel ke booking melalui port `Canceller`. Module `health` tidak memiliki tabel.

## 3. Peta module dan tanggung jawab

| Module | Tanggung jawab | Tabel yang dimiliki atau dibaca | Sumber utama |
|---|---|---|---|
| auth | register, OTP email, verifikasi, login, refresh rotation, logout, profil | `users`, `email_verifications`, `refresh_sessions` | `internal/module/auth` |
| trip | discovery, filter, detail, jadwal, destinasi, kategori | `trips`, `trip_schedules`, `trip_itineraries`, `trip_facilities`, `hotels`, `trip_hotels` | `internal/module/trip` |
| booking | seat lock, reservasi, peserta, promo, cancel | `reservations`, `participants`, `promo_codes`, `promo_redemptions` | `internal/module/booking` |
| document | upload, validasi, enkripsi, metadata, signed URL, hapus | `documents` | `internal/module/document` |
| payment | create charge, webhook HMAC, idempotency, transisi PAID, expiry payment | `payments`, `payment_events` | `internal/module/payment` |
| mytrip | read model reservasi, peserta, payment, dokumen, WA gating, timer, cancel | membaca `reservations`, `trips`, `trip_schedules`, `payments`, `participants`, `documents` | `internal/module/mytrip` |
| health | liveness dan readiness PostgreSQL/Redis | tidak ada | `internal/module/health` |

## 4. Alur bisnis

### 4.1 Register, OTP, login, dan refresh rotation

```mermaid
sequenceDiagram
    participant C as Client
    participant API as API Go
    participant DB as PostgreSQL
    participant M as Mock mailer

    C->>API: POST /api/v1/auth/register
    API->>DB: insert users status PENDING_VERIFICATION
    API->>DB: insert email_verifications code_hash
    API->>M: log kode OTP 6 digit
    API-->>C: 201 status PENDING_VERIFICATION

    C->>API: POST /api/v1/auth/verify-email
    API->>DB: cek code_hash expiry dan attempts
    API->>DB: consumed_at dan email_verified_at
    API->>DB: insert refresh_sessions
    API-->>C: 200 access token dan cookie HttpOnly

    C->>API: POST /api/v1/auth/login
    API->>DB: cek bcrypt dan email_verified_at
    API->>DB: insert refresh_sessions family baru
    API-->>C: 200 access token dan cookie HttpOnly

    C->>API: POST /api/v1/auth/refresh
    API->>DB: cari refresh_sessions berdasarkan token_hash
    API->>DB: insert sesi baru family yang sama
    API->>DB: revoked_at dan replaced_by pada sesi lama
    API-->>C: 200 access token baru dan cookie baru

    C->>API: POST /api/v1/auth/refresh dengan token lama
    API->>DB: sesi revoked terdeteksi
    API->>DB: revoke seluruh family
    API-->>C: 401 REFRESH_REUSE_DETECTED
```

Login sebelum verifikasi ditolak `403 EMAIL_NOT_VERIFIED`. Access token adalah JWT HS256; refresh token disimpan sebagai hash dan dikirim sebagai cookie HttpOnly. Reuse token lama mencabut seluruh family sesi.

### 4.2 Lock kursi dan seat hold Redis

```mermaid
sequenceDiagram
    participant C as Client
    participant B as Booking service
    participant DB as PostgreSQL
    participant R as Redis

    C->>B: POST /api/v1/reservations/lock
    B->>DB: cek schedule OPEN kuota dan harga
    B->>DB: cek hotel dan promo
    B->>R: Lua lock DECRBY seat:avail
    R->>R: SET seat:hold EX safety 1800
    R->>R: ZADD seat:holds score lock expiry 900

    alt kuota cukup
        B->>DB: insert reservations dan participants
        B->>DB: snapshot unit_price_idr dan total_amount_idr
        B-->>C: 201 reservation dan seats remaining
    else kuota habis
        B-->>C: 409 SEAT_UNAVAILABLE
    else user sudah punya reservasi aktif
        B-->>C: 409 RESERVATION_ALREADY_ACTIVE
    end

    Note over B,R: Sweeper 30 detik membaca DB lock_expires_at dan zset seat:holds
```

Key Redis yang dipakai:

| Key | Tipe | Fungsi |
|---|---|---|
| `seat:avail:{scheduleID}` | string integer | sisa seat yang belum di-hold dan belum terjual |
| `seat:hold:{reservationID}` | string integer | jumlah seat yang di-hold, TTL safety 1800 detik |
| `seat:holds:{scheduleID}` | sorted set | member reservationID, score unix expiry lock 900 detik |

Operasi lock, release, dan confirm-paid dijalankan sebagai Lua atomik. `payments` tidak pernah mengubah harga snapshot reservasi.

### 4.3 Upload dokumen, signed URL, dan gate kelengkapan

```mermaid
sequenceDiagram
    participant C as Client
    participant D as Document service
    participant FS as Storage lokal
    participant DB as PostgreSQL
    participant P as Payment service

    C->>D: POST /api/v1/documents multipart
    D->>D: cek owner participant reservasi PENDING dan lock aktif
    D->>D: validasi MIME maksimal 3 MB
    D->>FS: AES-256-GCM nonce 12 byte object key AAD
    D->>DB: insert documents metadata dan checksum
    D-->>C: 201 document id

    C->>D: GET /api/v1/documents/id/url
    D->>D: cek owner dan HMAC document id plus exp
    D-->>C: 200 signed URL 5 menit

    C->>D: GET /api/v1/documents/id/content dengan exp dan sig
    D->>D: verifikasi signature expiry dan checksum
    D->>FS: baca dan dekripsi
    D-->>C: 200 file

    C->>D: GET /api/v1/reservations/id/documents/completeness
    D->>DB: required_documents dan documents
    D-->>C: 200 complete atau missing

    P->>D: CompleteForReservation saat create payment
    D-->>P: true atau false
```

File disimpan di `UPLOADS_DIR/documents/reservations/{reservationID}/{documentID}.enc`. Nonce 12 byte ditulis di awal file, object key menjadi AAD, dan checksum SHA-256 plaintext disimpan di tabel `documents`. Tidak ada static route ke folder upload.

### 4.4 Payment create, webhook HMAC, dan transisi PAID

```mermaid
sequenceDiagram
    participant C as Client
    participant P as Mock provider
    participant S as Payment service
    participant DB as PostgreSQL
    participant R as Redis

    C->>S: POST /api/v1/reservations/id/payments
    S->>DB: cek owner reservasi PENDING dan lock aktif
    S->>DB: cek dokumen wajib lengkap
    S->>P: CreateCharge method dan amount_idr
    P-->>S: provider_ref VA atau QR string
    S->>DB: insert payments dan payment_expires_at 24 jam
    S-->>C: 201 payment PENDING

    P->>S: POST /api/v1/payments/webhook/mock
    S->>S: verifikasi HMAC X-Mock-Signature pada raw body
    alt signature tidak valid
        S-->>P: 400 INVALID_SIGNATURE
    else event baru
        S->>DB: insert payment_events ON CONFLICT DO NOTHING
        S->>DB: SELECT payments FOR UPDATE
        S->>DB: SELECT reservations dan trip_schedules FOR UPDATE
        S->>DB: seats_sold plus participants_count
        S->>DB: payments dan reservations menjadi PAID
        S->>R: ConfirmPaid hapus seat:hold tanpa kembalikan seat
        S-->>P: 200 received true
    else event duplikat
        S-->>P: 200 received true tanpa efek kedua
    end
```

Idempotency webhook memakai unique `payment_events(provider, provider_event_id)`. Signature diverifikasi sebelum repository menyentuh database. Payment PAID tidak mengembalikan seat; `seats_sold` permanen.

### 4.5 Expiry job reservasi dan payment

```mermaid
sequenceDiagram
    participant B as Booking sweeper
    participant E as Payment expiry job
    participant DB as PostgreSQL
    participant R as Redis

    loop setiap 30 detik
        B->>DB: SELECT reservations PENDING_PAYMENT lock_expires_at lewat
        B->>DB: UPDATE status EXPIRED dengan syarat PENDING_PAYMENT
        B->>R: Release seat hold
        B->>DB: fallback zset seat:holds yang sudah lewat
    end

    loop setiap 30 detik
        E->>DB: SELECT payments PENDING expires_at lewat FOR UPDATE SKIP LOCKED
        E->>DB: UPDATE payments EXPIRED
        alt reservasi masih PENDING_PAYMENT
            E->>DB: UPDATE reservations EXPIRED
            E->>R: Release seat hold
        else reservasi PAID
            E->>E: tidak menyentuh seat atau seats_sold
        end
    end
```

Kedua job idempoten: update memakai kondisi status, sehingga run berikutnya tidak mengembalikan seat dua kali. Reservasi yang sudah PAID tidak memenuhi filter expiry.

## 5. Topologi deployment

```mermaid
flowchart LR
    Client[Client] --> Proxy[Reverse proxy belum diuji]
    Proxy --> A1[API instance 1]
    Proxy --> A2[API instance 2]

    A1 --> PG[(PostgreSQL)]
    A2 --> PG
    A1 --> Redis[(Redis)]
    A2 --> Redis
    A1 --> FS[(Storage lokal bersama)]
    A2 --> FS

    Redis --> RL[Key rate limit rl route subject]
```

Satu binary API dapat dijalankan beberapa instance. Seat hold, rate limit, dan cooldown berbagi Redis; data transaksi berbagi PostgreSQL. Storage lokal memerlukan volume bersama bila instance lebih dari satu. Reverse proxy, TLS, dan header `X-Forwarded-*` belum diuji.

Key Redis untuk pembatas dan cooldown:

| Key | Fungsi |
|---|---|
| `rl:<route>:ip:<clientIP>` | fixed window rate limit per IP |
| `rl:<route>:user:<userID>` | fixed window rate limit per user |
| `cd:otp-resend:<email>` | cooldown kirim ulang OTP 15 detik |

## 6. Batasan dan keputusan kunci

| Topik | Keputusan |
|---|---|
| Uang | Semua nominal integer rupiah memakai field `*_idr`; tidak ada float untuk uang |
| Waktu | Database menyimpan UTC; tampilan dan tab mytrips memakai WIB |
| OTP | Mail driver mock menulis kode OTP ke log server; tidak mengirim email nyata |
| Payment | Mock provider aktif; adapter Midtrans adalah skeleton dan tidak didaftarkan di DI |
| Refund | Mock mengembalikan `ErrNotSupported`; belum ada endpoint refund |
| Dokumen | Storage lokal AES-256-GCM; S3 belum diimplementasikan |
| Frontend | Belum ada; API dipakai klien web atau mobile di masa depan |
| Rate limit | Redis fixed window lintas instance; limit saat ini hardcoded di handler |
| Cancel | Sebelum bayar boleh; setelah PAID ditolak `409 ALREADY_PAID` |
| WA | `join_wa_url` hanya ada untuk reservasi PAID |

## 7. Verifikasi nama dari kode

| Yang diverifikasi | File sumber |
|---|---|
| Route health, auth, trip, booking, document, payment, mytrip | `internal/api/router.go`, `internal/module/*/handler/handler.go` |
| Key dan operasi Lua seat hold | `internal/infra/redis/seathold.go` |
| Key rate limit dan cooldown | `internal/infra/ratelimit/limiter.go`, `internal/module/auth/handler/handler.go` |
| Tabel auth | `internal/infra/database/migration/00001_init_auth.sql` |
| Tabel trip dan jadwal | `internal/infra/database/migration/00002_init_trips.sql` |
| Tabel promo, reservasi, peserta | `internal/infra/database/migration/00003_init_promos.sql`, `00004_init_booking.sql` |
| Tabel payment dan event | `internal/infra/database/migration/00005_init_payments.sql` |
| Tabel dokumen | `internal/infra/database/migration/00007_init_documents.sql` |
| Alur lock dan expiry reservasi | `internal/module/booking/service/service.go`, `internal/module/booking/repository/repository_impl.go` |
| Alur dokumen dan completeness | `internal/module/document/service/service.go`, `internal/module/document/handler/handler.go` |
| Alur payment dan webhook | `internal/module/payment/service/service.go`, `internal/module/payment/repository/repository_impl.go` |
| Alur mytrip dan WA gating | `internal/module/mytrip/service/service.go`, `internal/module/mytrip/repository/repository_impl.go` |
| Middleware keamanan dan CORS | `internal/api/middleware/*.go` |
| Konfigurasi TTL dan limit | `internal/config/config.go`, `internal/api/middleware/rate_limit.go` |

Dokumen ini tidak mengubah kontrak API. Kontrak lengkap ada di `internal/api/openapi.json` dan disajikan di `GET /openapi.json`.