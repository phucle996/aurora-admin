# Configuration

Aurora Admin đọc config qua env file (thường là `/etc/aurora-admin.env`).

## Required

- `APP_HOSTNAME`
- `ETCD_ENDPOINTS`
- `DATABASE_URL`

## Core App

- `APP_HOSTNAME`: domain public của Admin (ví dụ `admin.aurora.local`)
- `APP_PORT`: backend port nội bộ (installer sẽ random và ghi lại)
- `APP_LOG_LEVEL`: `debug|info|warn|error`
- `APP_TIMEZONE`: ví dụ `Asia/Ho_Chi_Minh`

## Agent mTLS

- `APP_AGENT_TLS_CA_CERT_FILE`
- `APP_AGENT_TLS_CA_KEY_FILE`
- `APP_AGENT_TLS_ADMIN_CLIENT_CERT_FILE`
- `APP_AGENT_TLS_ADMIN_CLIENT_KEY_FILE`

Các path mặc định do installer tạo trong `/etc/aurora/certs`.

## etcd

- `ETCD_ENDPOINTS`
- `ETCD_DIAL_TIMEOUT`
- `ETCD_TLS` + cert options nếu dùng TLS
- `ETCD_USERNAME`, `ETCD_PASSWORD` (optional)

## Database / Redis

- `DATABASE_URL`
- `DB_SSLMODE`
- `REDIS_ADDR`
- `REDIS_*` options (optional theo runtime)

## Telegram (optional)

- `TELEGRAM_ENABLE`
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_CHAT_ID`

## Runtime bootstrap behavior

Khi startup:

1. Load env.
2. Connect etcd.
3. Seed missing runtime keys.
4. Reload config từ etcd.
5. Start HTTP + gRPC.
