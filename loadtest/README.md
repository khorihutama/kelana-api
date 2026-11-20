# Load test Kelana API

Skrip k6 untuk discovery read-heavy, login, dan lock/cancel.

```bash
k6 run loadtest/k6.js
```

Dengan alur auth + booking:

```bash
LOADTEST_EMAIL=... LOADTEST_PASSWORD=... \
  k6 run loadtest/k6.js
```

Variabel opsional:

| Env | Default | Arti |
|---|---|---|
| `BASE_URL` | `http://127.0.0.1:8080` | Alamat API |
| `LOADTEST_VUS` | `20` | VU discovery |
| `LOADTEST_DURATION` | `30s` | Durasi discovery |
| `LOADTEST_EMAIL` / `LOADTEST_PASSWORD` | kosong | Mengaktifkan skenario login dan booking |

Ambang: p95 discovery < 800 ms, `http_req_failed` < 2%, `business_errors` < 2%. Login dan booking memakai 3/5 iterasi agar berada di bawah rate limit 5 login/menit dan 10 lock/menit per user.