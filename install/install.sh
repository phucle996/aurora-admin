#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

REPO="${AURORA_ADMIN_GITHUB_REPO:-phucle996/aurora-admin}"
VERSION="${AURORA_ADMIN_VERSION:-latest}"
APP_NAME="${AURORA_ADMIN_BIN_NAME:-aurora-admin-service}"
INSTALL_DIR="${AURORA_ADMIN_INSTALL_DIR:-/usr/local/bin}"
BIN_PATH="${INSTALL_DIR}/${APP_NAME}"

SERVICE_NAME="${AURORA_ADMIN_SERVICE_NAME:-aurora-admin.service}"
SERVICE_PATH="/etc/systemd/system/${SERVICE_NAME}"
ENV_FILE="${AURORA_ADMIN_ENV_FILE:-/etc/aurora-admin.env}"
SERVICE_USER="${AURORA_ADMIN_SERVICE_USER:-aurora}"
SERVICE_GROUP="${AURORA_ADMIN_SERVICE_GROUP:-aurora}"
SERVICE_HOME="${AURORA_ADMIN_SERVICE_HOME:-/var/lib/aurora}"
TLS_DIR="${AURORA_ADMIN_TLS_DIR:-/etc/aurora/certs}"
TLS_CERT_FILE="${TLS_DIR}/admin.crt"
TLS_KEY_FILE="${TLS_DIR}/admin.key"
TLS_CA_FILE="${TLS_DIR}/ca.crt"
TLS_CA_KEY_FILE="${TLS_DIR}/ca.key"
AGENT_TLS_CA_FILE="${TLS_DIR}/agent-ca.crt"
AGENT_TLS_CA_KEY_FILE="${TLS_DIR}/agent-ca.key"
AGENT_TLS_ADMIN_CLIENT_CERT_FILE="${TLS_DIR}/admin-agent-client.crt"
AGENT_TLS_ADMIN_CLIENT_KEY_FILE="${TLS_DIR}/admin-agent-client.key"
TLS_NGINX_CLIENT_CERT_FILE="${TLS_DIR}/nginx-client.crt"
TLS_NGINX_CLIENT_KEY_FILE="${TLS_DIR}/nginx-client.key"
NGINX_SERVICE_NAME="${AURORA_ADMIN_NGINX_SERVICE_NAME:-nginx}"
NGINX_CONF_FILE="${AURORA_ADMIN_NGINX_CONF_FILE:-/etc/nginx/conf.d/aurora-admin.conf}"
NGINX_TEMPLATE_FILE="${AURORA_ADMIN_NGINX_TEMPLATE_FILE:-${SCRIPT_DIR}/nginx.conf}"
NGINX_TEMPLATE_URL="${AURORA_ADMIN_NGINX_TEMPLATE_URL:-}"
NGINX_CACHE_DIR="${AURORA_ADMIN_NGINX_CACHE_DIR:-/var/cache/nginx/aurora-admin}"
BACKEND_PORT_MIN="${AURORA_ADMIN_BACKEND_PORT_MIN:-20000}"
BACKEND_PORT_MAX="${AURORA_ADMIN_BACKEND_PORT_MAX:-60000}"

CONFIG_OUTPUT="${AURORA_ADMIN_CONFIG_OUTPUT:-./aurora-admin.env.sample}"
INPUT_ENV_FILE=""
MODE="install"
TMP_DIR=""
DELETE_INPUT_ENV="true"
BACKEND_PORT=""

log() { printf '[install] %s\n' "$1"; }
die() { printf '[install][error] %s\n' "$1" >&2; exit 1; }
warn() { printf '[install][warn] %s\n' "$1" >&2; }

cleanup() {
  if [ -n "${TMP_DIR}" ] && [ -d "${TMP_DIR}" ]; then
    rm -rf "${TMP_DIR}" || true
  fi
}

as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    die "need root permission (run as root or install sudo)"
  fi
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "missing command: $1"
}

usage() {
  cat <<'USAGE'
Usage:
  ./install.sh --config [output_file]
  ./install.sh -f <env_file> [-v <release_tag>]

Options:
  --config [output_file]  Generate sample env file then exit.
  -f, --file <env_file>   Install using provided env file.
  -v, --version <tag>     Install a specific GitHub release tag.
  --keep-env-file         Do not delete input env file after success.
  -h, --help              Show help.
USAGE
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --config)
        MODE="config"
        if [ "$#" -gt 1 ] && [ "${2#-}" != "$2" ]; then
          :
        elif [ "$#" -gt 1 ] && [ "${2}" != "" ]; then
          CONFIG_OUTPUT="$2"
          shift
        fi
        ;;
      -f|--file)
        [ "$#" -gt 1 ] || die "missing value for $1"
        INPUT_ENV_FILE="$2"
        shift
        ;;
      -v|--version)
        [ "$#" -gt 1 ] || die "missing value for $1"
        VERSION="$2"
        shift
        ;;
      --keep-env-file)
        DELETE_INPUT_ENV="false"
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
    shift
  done

  if [ "$MODE" = "config" ] && [ -n "$INPUT_ENV_FILE" ]; then
    die "cannot use --config with -f/--file"
  fi
}

arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) die "unsupported architecture: $(uname -m)" ;;
  esac
}

download() {
  local url="$1"
  local out="$2"
  local token="${AURORA_ADMIN_GITHUB_TOKEN:-}"

  if command -v curl >/dev/null 2>&1; then
    if [ -n "$token" ]; then
      curl -fsSL -H "Authorization: Bearer ${token}" "$url" -o "$out"
    else
      curl -fsSL "$url" -o "$out"
    fi
    return
  fi

  if command -v wget >/dev/null 2>&1; then
    if [ -n "$token" ]; then
      wget --quiet --header="Authorization: Bearer ${token}" "$url" -O "$out"
    else
      wget --quiet "$url" -O "$out"
    fi
    return
  fi

  die "need curl or wget to download release assets"
}

resolve_tag() {
  if [ "$VERSION" != "latest" ]; then
    echo "$VERSION"
    return
  fi

  local api_json="${TMP_DIR}/latest.json"
  download "https://api.github.com/repos/${REPO}/releases/latest" "$api_json"
  sed -n 's/.*"tag_name":[[:space:]]*"\([^"]\+\)".*/\1/p' "$api_json" | head -n1
}

verify_checksum() {
  local tar_file="$1"
  local checksums="$2"
  local asset="$3"

  local expected
  expected="$(grep -E "[[:space:]](dist/)?${asset}$" "$checksums" | awk '{print $1}' | head -n1 || true)"
  if [ -z "$expected" ]; then
    warn "skip checksum: ${asset} not found in checksums.txt"
    return
  fi

  local actual
  actual="$(sha256sum "$tar_file" | awk '{print $1}')"
  [ "$actual" = "$expected" ] || die "checksum mismatch for ${asset}"
}

read_env_value() {
  local key="$1"
  local file="$2"
  local line

  if [ -r "$file" ]; then
    line="$(awk -F= -v k="$key" '$1 == k { print substr($0, index($0, "=") + 1) }' "$file" | tail -n1 || true)"
  else
    line="$(as_root awk -F= -v k="$key" '$1 == k { print substr($0, index($0, "=") + 1) }' "$file" | tail -n1 || true)"
  fi

  if [ -z "$line" ]; then
    printf '%s' ""
    return
  fi
  line="${line#*=}"
  line="${line%\"}"
  line="${line#\"}"
  printf '%s' "$line"
}

ensure_service_group() {
  if getent group "$SERVICE_GROUP" >/dev/null 2>&1; then
    return
  fi

  if command -v groupadd >/dev/null 2>&1; then
    as_root groupadd --system "$SERVICE_GROUP"
    return
  fi
  if command -v addgroup >/dev/null 2>&1; then
    as_root addgroup --system "$SERVICE_GROUP"
    return
  fi

  die "cannot create service group: neither groupadd nor addgroup found"
}

ensure_service_user() {
  ensure_service_group

  if id -u "$SERVICE_USER" >/dev/null 2>&1; then
    as_root mkdir -p "$SERVICE_HOME"
    as_root chown "${SERVICE_USER}:${SERVICE_GROUP}" "$SERVICE_HOME"
    as_root chmod 700 "$SERVICE_HOME"
    return
  fi

  local no_login_shell="/usr/sbin/nologin"
  [ -x "$no_login_shell" ] || no_login_shell="/sbin/nologin"
  [ -x "$no_login_shell" ] || no_login_shell="/bin/false"

  if command -v useradd >/dev/null 2>&1; then
    as_root useradd \
      --system \
      --create-home \
      --home-dir "$SERVICE_HOME" \
      --gid "$SERVICE_GROUP" \
      --shell "$no_login_shell" \
      "$SERVICE_USER"
  elif command -v adduser >/dev/null 2>&1; then
    as_root adduser \
      --system \
      --home "$SERVICE_HOME" \
      --shell "$no_login_shell" \
      --ingroup "$SERVICE_GROUP" \
      "$SERVICE_USER"
  else
    die "cannot create service user: neither useradd nor adduser found"
  fi

  as_root mkdir -p "$SERVICE_HOME"
  as_root chown "${SERVICE_USER}:${SERVICE_GROUP}" "$SERVICE_HOME"
  as_root chmod 700 "$SERVICE_HOME"
}

ensure_service_user_sudo_group() {
  # Allow runtime process user to run sudo when explicitly required by module-install flows.
  if ! getent group sudo >/dev/null 2>&1; then
    if command -v groupadd >/dev/null 2>&1; then
      as_root groupadd sudo || true
    elif command -v addgroup >/dev/null 2>&1; then
      as_root addgroup sudo || true
    fi
  fi

  if getent group sudo >/dev/null 2>&1; then
    if id -nG "$SERVICE_USER" | tr ' ' '\n' | grep -qx "sudo"; then
      return
    fi
    if command -v usermod >/dev/null 2>&1; then
      as_root usermod -aG sudo "$SERVICE_USER"
      return
    fi
    if command -v adduser >/dev/null 2>&1; then
      as_root adduser "$SERVICE_USER" sudo
      return
    fi
    warn "cannot add ${SERVICE_USER} to sudo group automatically"
  fi
}

ensure_tls_dir() {
  as_root mkdir -p "$TLS_DIR"
  as_root chown root:"$SERVICE_GROUP" "$TLS_DIR"
  as_root chmod 750 "$TLS_DIR"
}

port_in_use() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltnH | awk '{print $4}' | grep -Eq "[:.]${port}$"
    return $?
  fi
  if command -v netstat >/dev/null 2>&1; then
    netstat -ltn 2>/dev/null | awk '{print $4}' | grep -Eq "[:.]${port}$"
    return $?
  fi
  return 1
}

random_backend_port() {
  local min max attempt candidate
  min="$BACKEND_PORT_MIN"
  max="$BACKEND_PORT_MAX"
  [ "$min" -lt "$max" ] || die "invalid backend port range: ${min}-${max}"

  for attempt in $(seq 1 128); do
    candidate=$(( RANDOM % (max - min + 1) + min ))
    if ! port_in_use "$candidate"; then
      echo "$candidate"
      return
    fi
  done

  die "cannot find free backend port in range ${min}-${max}"
}

upsert_env_value() {
  local file="$1"
  local key="$2"
  local value="$3"
  local tmp_file="${TMP_DIR}/env-${key}.tmp"

  as_root awk -v k="$key" -v v="$value" '
    BEGIN { updated=0 }
    $0 ~ "^" k "=" {
      print k "=" v
      updated=1
      next
    }
    { print }
    END {
      if (updated == 0) {
        print k "=" v
      }
    }
  ' "$file" > "$tmp_file"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$tmp_file" "$file"
}

assign_random_backend_port() {
  BACKEND_PORT="$(random_backend_port)"
  upsert_env_value "$ENV_FILE" "APP_PORT" "$BACKEND_PORT"
  log "assigned random backend APP_PORT=${BACKEND_PORT}"
}

write_config_template() {
  local out="$1"
  cat > "$out" <<'ENV_TPL'
# Aurora Admin Service env template
# Fill all required values before install.

APP_HOSTNAME=aurora-admin.local
APP_PORT=3009
APP_LOG_LEVEL=info
APP_TIMEZONE=Asia/Ho_Chi_Minh
APP_AGENT_TLS_CA_CERT_FILE=/etc/aurora/certs/agent-ca.crt
APP_AGENT_TLS_CA_KEY_FILE=/etc/aurora/certs/agent-ca.key
APP_AGENT_TLS_ADMIN_CLIENT_CERT_FILE=/etc/aurora/certs/admin-agent-client.crt
APP_AGENT_TLS_ADMIN_CLIENT_KEY_FILE=/etc/aurora/certs/admin-agent-client.key

ETCD_ENDPOINTS=127.0.0.1:2379
ETCD_AUTO_SYNC_INTERVAL=5m
ETCD_DIAL_TIMEOUT=5s
ETCD_DIAL_KEEPALIVE_TIME=30s
ETCD_DIAL_KEEPALIVE_TIMEOUT=10s
ETCD_USERNAME=
ETCD_PASSWORD=
ETCD_TLS=false
ETCD_TLS_CA=
ETCD_TLS_CERT=
ETCD_TLS_KEY=
ETCD_TLS_SERVER_NAME=
ETCD_TLS_INSECURE=false
ETCD_PERMIT_WITHOUT_STREAM=false
ETCD_REJECT_OLD_CLUSTER=false
ETCD_MAX_CALL_SEND_MSG_SIZE=2097152
ETCD_MAX_CALL_RECV_MSG_SIZE=2097152

DATABASE_URL=
DB_SSLMODE=disable

REDIS_ADDR=127.0.0.1:6379
REDIS_USERNAME=
REDIS_PASSWORD=
REDIS_DB=0
REDIS_TLS=false
REDIS_TLS_CA=
REDIS_TLS_CERT=
REDIS_TLS_KEY=
REDIS_TLS_INSECURE=false

TELEGRAM_ENABLE=false
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=

ENV_TPL
  chmod 600 "$out"
}

install_binary() {
  local tag="$1"
  local machine_arch="$2"
  local asset="${APP_NAME}_linux_${machine_arch}.tar.gz"
  local tar_file="${TMP_DIR}/${asset}"
  local checksums="${TMP_DIR}/checksums.txt"
  local base_url="https://github.com/${REPO}/releases/download/${tag}"

  log "download ${asset} (${REPO}@${tag})"
  download "${base_url}/${asset}" "$tar_file"
  download "${base_url}/checksums.txt" "$checksums"
  verify_checksum "$tar_file" "$checksums" "$asset"

  tar -xzf "$tar_file" -C "$TMP_DIR"

  local extracted="${TMP_DIR}/${APP_NAME}_linux_${machine_arch}"
  [ -f "$extracted" ] || extracted="${TMP_DIR}/${APP_NAME}"
  [ -f "$extracted" ] || die "binary not found in archive ${asset}"

  as_root mkdir -p "$INSTALL_DIR"
  as_root install -m 0755 -o root -g root "$extracted" "$BIN_PATH"
}

install_env_file() {
  local source_env="$1"
  [ -f "$source_env" ] || die "env file not found: ${source_env}"

  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$source_env" "$ENV_FILE"
}

install_tls_materials() {
  local env_file="$1"
  local app_host cert_tmp key_tmp ca_tmp ca_key_tmp csr_tmp ext_tmp
  local agent_ca_tmp agent_ca_key_tmp agent_client_key_tmp agent_client_csr_tmp agent_client_ext_tmp agent_client_cert_tmp
  local nginx_key_tmp nginx_csr_tmp nginx_ext_tmp nginx_cert_tmp
  ensure_tls_dir
  app_host="$(read_env_value "APP_HOSTNAME" "$env_file")"
  [ -n "$app_host" ] || app_host="aurora-admin.local"

  cert_tmp="${TMP_DIR}/admin.crt"
  key_tmp="${TMP_DIR}/admin.key"
  ca_tmp="${TMP_DIR}/ca.crt"
  ca_key_tmp="${TMP_DIR}/ca.key"
  csr_tmp="${TMP_DIR}/admin.csr"
  ext_tmp="${TMP_DIR}/admin.ext"
  agent_ca_tmp="${TMP_DIR}/agent-ca.crt"
  agent_ca_key_tmp="${TMP_DIR}/agent-ca.key"
  agent_client_key_tmp="${TMP_DIR}/admin-agent-client.key"
  agent_client_csr_tmp="${TMP_DIR}/admin-agent-client.csr"
  agent_client_ext_tmp="${TMP_DIR}/admin-agent-client.ext"
  agent_client_cert_tmp="${TMP_DIR}/admin-agent-client.crt"
  nginx_key_tmp="${TMP_DIR}/nginx-client.key"
  nginx_csr_tmp="${TMP_DIR}/nginx-client.csr"
  nginx_ext_tmp="${TMP_DIR}/nginx-client.ext"
  nginx_cert_tmp="${TMP_DIR}/nginx-client.crt"

  log "generate self-signed tls cert/key/ca (edge + agent mTLS)"
  openssl genrsa -out "$ca_key_tmp" 4096 >/dev/null 2>&1
  openssl req -x509 -new -nodes \
    -key "$ca_key_tmp" \
    -sha256 \
    -days 3650 \
    -out "$ca_tmp" \
    -subj "/CN=Aurora Admin CA" >/dev/null 2>&1

  openssl genrsa -out "$key_tmp" 2048 >/dev/null 2>&1
  openssl req -new \
    -key "$key_tmp" \
    -out "$csr_tmp" \
    -subj "/CN=${app_host}" >/dev/null 2>&1

  cat > "$ext_tmp" <<EOF
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = DNS:${app_host},DNS:localhost,IP:127.0.0.1
EOF

  openssl x509 -req \
    -in "$csr_tmp" \
    -CA "$ca_tmp" \
    -CAkey "$ca_key_tmp" \
    -CAcreateserial \
    -out "$cert_tmp" \
    -days 825 \
    -sha256 \
    -extfile "$ext_tmp" >/dev/null 2>&1

  openssl genrsa -out "$agent_ca_key_tmp" 4096 >/dev/null 2>&1
  openssl req -x509 -new -nodes \
    -key "$agent_ca_key_tmp" \
    -sha256 \
    -days 3650 \
    -out "$agent_ca_tmp" \
    -subj "/CN=Aurora Agent mTLS CA" >/dev/null 2>&1

  openssl genrsa -out "$agent_client_key_tmp" 2048 >/dev/null 2>&1
  openssl req -new \
    -key "$agent_client_key_tmp" \
    -out "$agent_client_csr_tmp" \
    -subj "/CN=aurora-admin-agent-client" >/dev/null 2>&1

cat > "$agent_client_ext_tmp" <<EOF
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
subjectAltName = DNS:aurora-admin-agent-client,DNS:${app_host},DNS:localhost,IP:127.0.0.1,URI:spiffe://aurora.local/service/aurora-admin,URI:spiffe://aurora.local/role/control-plane,URI:spiffe://aurora.local/node/admin-control-plane
EOF

  openssl x509 -req \
    -in "$agent_client_csr_tmp" \
    -CA "$agent_ca_tmp" \
    -CAkey "$agent_ca_key_tmp" \
    -CAcreateserial \
    -out "$agent_client_cert_tmp" \
    -days 825 \
    -sha256 \
    -extfile "$agent_client_ext_tmp" >/dev/null 2>&1

  openssl genrsa -out "$nginx_key_tmp" 2048 >/dev/null 2>&1
  openssl req -new \
    -key "$nginx_key_tmp" \
    -out "$nginx_csr_tmp" \
    -subj "/CN=aurora-nginx" >/dev/null 2>&1

  cat > "$nginx_ext_tmp" <<EOF
basicConstraints=CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
subjectAltName = DNS:aurora-nginx,DNS:localhost,IP:127.0.0.1
EOF

  openssl x509 -req \
    -in "$nginx_csr_tmp" \
    -CA "$ca_tmp" \
    -CAkey "$ca_key_tmp" \
    -CAcreateserial \
    -out "$nginx_cert_tmp" \
    -days 825 \
    -sha256 \
    -extfile "$nginx_ext_tmp" >/dev/null 2>&1

  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$cert_tmp" "$TLS_CERT_FILE"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$key_tmp" "$TLS_KEY_FILE"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$ca_tmp" "$TLS_CA_FILE"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$ca_key_tmp" "$TLS_CA_KEY_FILE"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$agent_ca_tmp" "$AGENT_TLS_CA_FILE"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$agent_ca_key_tmp" "$AGENT_TLS_CA_KEY_FILE"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$agent_client_cert_tmp" "$AGENT_TLS_ADMIN_CLIENT_CERT_FILE"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$agent_client_key_tmp" "$AGENT_TLS_ADMIN_CLIENT_KEY_FILE"
  as_root install -m 0444 -o root -g root "$nginx_cert_tmp" "$TLS_NGINX_CLIENT_CERT_FILE"
  as_root install -m 0400 -o root -g root "$nginx_key_tmp" "$TLS_NGINX_CLIENT_KEY_FILE"
}

preflight_tls_materials() {
  log "preflight tls materials"
  as_root test -d "$TLS_DIR" || die "tls preflight failed: missing dir $TLS_DIR"
  for path in \
    "$TLS_CERT_FILE" "$TLS_KEY_FILE" "$TLS_CA_FILE" "$TLS_CA_KEY_FILE" \
    "$AGENT_TLS_CA_FILE" "$AGENT_TLS_CA_KEY_FILE" "$AGENT_TLS_ADMIN_CLIENT_CERT_FILE" "$AGENT_TLS_ADMIN_CLIENT_KEY_FILE" \
    "$TLS_NGINX_CLIENT_CERT_FILE" "$TLS_NGINX_CLIENT_KEY_FILE"; do
    as_root test -s "$path" || die "tls preflight failed: missing file $path"
  done

  local cert_check key_check ca_check verify_check cert_mod key_mod
  local agent_ca_check agent_ca_key_check agent_verify_check agent_client_mod agent_client_key_mod
  cert_check="$(as_root openssl x509 -in "$TLS_CERT_FILE" -noout 2>&1)" || die "tls preflight failed: invalid cert ${TLS_CERT_FILE}: ${cert_check}"
  key_check="$(as_root openssl rsa -in "$TLS_KEY_FILE" -check -noout 2>&1)" || die "tls preflight failed: invalid key ${TLS_KEY_FILE}: ${key_check}"
  ca_check="$(as_root openssl x509 -in "$TLS_CA_FILE" -noout 2>&1)" || die "tls preflight failed: invalid ca ${TLS_CA_FILE}: ${ca_check}"

  verify_check="$(as_root openssl verify -CAfile "$TLS_CA_FILE" "$TLS_CERT_FILE" 2>&1)" || die "tls preflight failed: cert is not signed by ca: ${verify_check}"
  cert_mod="$(as_root openssl x509 -noout -modulus -in "$TLS_CERT_FILE" | openssl md5 2>/dev/null | awk '{print $NF}')"
  key_mod="$(as_root openssl rsa -noout -modulus -in "$TLS_KEY_FILE" | openssl md5 2>/dev/null | awk '{print $NF}')"
  [ -n "$cert_mod" ] || die "tls preflight failed: cannot read cert modulus"
  [ -n "$key_mod" ] || die "tls preflight failed: cannot read key modulus"
  [ "$cert_mod" = "$key_mod" ] || die "tls preflight failed: cert/key mismatch"

  agent_ca_check="$(as_root openssl x509 -in "$AGENT_TLS_CA_FILE" -noout 2>&1)" || die "tls preflight failed: invalid agent ca ${AGENT_TLS_CA_FILE}: ${agent_ca_check}"
  agent_ca_key_check="$(as_root openssl rsa -in "$AGENT_TLS_CA_KEY_FILE" -check -noout 2>&1)" || die "tls preflight failed: invalid agent ca key ${AGENT_TLS_CA_KEY_FILE}: ${agent_ca_key_check}"
  agent_verify_check="$(as_root openssl verify -CAfile "$AGENT_TLS_CA_FILE" "$AGENT_TLS_ADMIN_CLIENT_CERT_FILE" 2>&1)" || die "tls preflight failed: admin agent client cert is not signed by agent ca: ${agent_verify_check}"
  agent_client_mod="$(as_root openssl x509 -noout -modulus -in "$AGENT_TLS_ADMIN_CLIENT_CERT_FILE" | openssl md5 2>/dev/null | awk '{print $NF}')"
  agent_client_key_mod="$(as_root openssl rsa -noout -modulus -in "$AGENT_TLS_ADMIN_CLIENT_KEY_FILE" | openssl md5 2>/dev/null | awk '{print $NF}')"
  [ -n "$agent_client_mod" ] || die "tls preflight failed: cannot read admin agent client cert modulus"
  [ -n "$agent_client_key_mod" ] || die "tls preflight failed: cannot read admin agent client key modulus"
  [ "$agent_client_mod" = "$agent_client_key_mod" ] || die "tls preflight failed: admin agent client cert/key mismatch"
}

ensure_nginx_installed() {
  if command -v nginx >/dev/null 2>&1; then
    return
  fi

  log "install nginx"
  if command -v apt-get >/dev/null 2>&1; then
    as_root apt-get install -y nginx
    return
  fi
  if command -v dnf >/dev/null 2>&1; then
    as_root dnf install -y nginx
    return
  fi
  if command -v yum >/dev/null 2>&1; then
    as_root yum install -y nginx
    return
  fi
  if command -v apk >/dev/null 2>&1; then
    as_root apk add --no-cache nginx
    return
  fi

  die "cannot install nginx automatically (unsupported package manager)"
}

ensure_nginx_cache_dir() {
  local nginx_user
  nginx_user="$(as_root awk '/^[[:space:]]*user[[:space:]]+/ {gsub(/;/,"",$2); print $2; exit}' /etc/nginx/nginx.conf 2>/dev/null || true)"
  if [ -z "$nginx_user" ]; then
    nginx_user="www-data"
  fi

  as_root mkdir -p "$NGINX_CACHE_DIR"
  as_root chown -R "${nginx_user}:${nginx_user}" "$NGINX_CACHE_DIR" || as_root chown -R "${nginx_user}" "$NGINX_CACHE_DIR"
  as_root chmod 750 "$NGINX_CACHE_DIR" || true
}

nginx_supports_brotli() {
  if as_root nginx -V 2>&1 | grep -qi "brotli"; then
    return 0
  fi
  if as_root sh -lc 'ls /etc/nginx/modules-enabled/*brotli*.conf >/dev/null 2>&1'; then
    return 0
  fi
  if as_root sh -lc 'ls /usr/lib/nginx/modules/*brotli*.so >/dev/null 2>&1'; then
    return 0
  fi
  return 1
}

render_nginx_template() {
  local src="$1"
  local dst="$2"
  local app_host="$3"
  local backend_port="$4"
  local brotli_enabled="0"

  if nginx_supports_brotli; then
    sed '/^## __BROTLI_BEGIN__$/d; /^## __BROTLI_END__$/d' "$src" > "$dst"
    log "nginx brotli: enabled"
    brotli_enabled="1"
  else
    sed '/^## __BROTLI_BEGIN__$/,/^## __BROTLI_END__$/d' "$src" > "$dst"
    log "nginx brotli: not available, fallback to gzip"
  fi

  sed -i \
    -e "s|__SERVER_NAME__|${app_host}|g" \
    -e "s|__BACKEND_PORT__|${backend_port}|g" \
    -e "s|__TLS_CERT_FILE__|${TLS_CERT_FILE}|g" \
    -e "s|__TLS_KEY_FILE__|${TLS_KEY_FILE}|g" \
    -e "s|__TLS_CA_FILE__|${TLS_CA_FILE}|g" \
    -e "s|__TLS_NGINX_CLIENT_CERT_FILE__|${TLS_NGINX_CLIENT_CERT_FILE}|g" \
    -e "s|__TLS_NGINX_CLIENT_KEY_FILE__|${TLS_NGINX_CLIENT_KEY_FILE}|g" \
    -e "s|__NGINX_CACHE_DIR__|${NGINX_CACHE_DIR}|g" \
    "$dst"

  normalize_nginx_compression "$dst" "$brotli_enabled"
}

normalize_nginx_compression() {
  local file="$1"
  local brotli_enabled="$2"
  local tmp_file="${TMP_DIR}/aurora-admin-nginx.normalized.conf"

  awk -v brotli="$brotli_enabled" '
    BEGIN {
      seen_server = 0
      skip_global_types = 0
      injected = 0
    }
    {
      line = $0

      if (line ~ /^[[:space:]]*server[[:space:]]*\{/) {
        seen_server = 1
      }

      if (!seen_server) {
        if (skip_global_types) {
          if (line ~ /;/) {
            skip_global_types = 0
          }
          next
        }
        if (line ~ /^[[:space:]]*(brotli_types|gzip_types)[[:space:]]*$/ || line ~ /^[[:space:]]*(brotli_types|gzip_types)[[:space:]]+/) {
          if (line !~ /;/) {
            skip_global_types = 1
          }
          next
        }
        if (line ~ /^[[:space:]]*(brotli(_[a-z_]+)?|gzip(_[a-z_]+)?)[[:space:]]+/) {
          next
        }
      }

      print line

      if (!injected && line ~ /^[[:space:]]*ssl_prefer_server_ciphers[[:space:]]+off;/) {
        print ""
        if (brotli == "1") {
          print "  brotli on;"
          print "  brotli_static on;"
          print "  brotli_comp_level 5;"
          print "  brotli_min_length 512;"
          print "  brotli_types text/plain text/css text/xml text/javascript application/javascript application/x-javascript application/json application/xml application/rss+xml application/wasm image/svg+xml;"
        }
        print "  gzip on;"
        print "  gzip_vary on;"
        print "  gzip_comp_level 5;"
        print "  gzip_min_length 512;"
        print "  gzip_proxied any;"
        print "  gzip_types text/plain text/css text/xml text/javascript application/javascript application/x-javascript application/json application/xml application/rss+xml application/wasm image/svg+xml;"
        injected = 1
      }
    }
  ' "$file" > "$tmp_file"

  mv "$tmp_file" "$file"
}

migrate_nginx_http2_deprecated_file() {
  local file="$1"
  [ -f "$file" ] || return 0

  local tmp_file="${TMP_DIR}/$(basename "$file").http2fix"
  awk '
    BEGIN {
      in_server = 0
      depth = 0
      has_http2 = 0
      needs_http2 = 0
    }
    {
      line = $0
      if (!in_server && line ~ /^[[:space:]]*server[[:space:]]*\{[[:space:]]*$/) {
        in_server = 1
        depth = 0
        has_http2 = 0
        needs_http2 = 0
      }

      if (in_server && line ~ /^[[:space:]]*http2[[:space:]]+on;[[:space:]]*$/) {
        has_http2 = 1
      }
      if (in_server && line ~ /^[[:space:]]*listen[[:space:]]+443[[:space:]]+ssl[[:space:]]+http2;[[:space:]]*$/) {
        sub(/[[:space:]]+http2;/, ";", line)
        needs_http2 = 1
      }
      if (in_server && line ~ /^[[:space:]]*listen[[:space:]]+\[::\]:443[[:space:]]+ssl[[:space:]]+http2;[[:space:]]*$/) {
        sub(/[[:space:]]+http2;/, ";", line)
        needs_http2 = 1
      }

      if (in_server && depth == 1 && line ~ /^[[:space:]]*}[[:space:]]*$/) {
        if (needs_http2 && !has_http2) {
          print "  http2 on;"
          has_http2 = 1
        }
      }

      print line

      if (in_server) {
        open_count = gsub(/\{/, "{", line)
        close_count = gsub(/\}/, "}", line)
        depth += open_count
        depth -= close_count
        if (depth <= 0) {
          in_server = 0
          depth = 0
          has_http2 = 0
          needs_http2 = 0
        }
      }
    }
  ' "$file" > "$tmp_file"
  as_root install -m 0644 -o root -g root "$tmp_file" "$file"
}

migrate_nginx_http2_deprecated_all() {
  local conf
  for conf in /etc/nginx/conf.d/aurora-*.conf; do
    [ -e "$conf" ] || continue
    migrate_nginx_http2_deprecated_file "$conf"
  done
}

ensure_nginx_template_file() {
  local release_tag="$1"
  local conf_tmp fetch_url fallback_url
  conf_tmp="${TMP_DIR}/aurora-admin-nginx.template.conf"

  if [ -n "$NGINX_TEMPLATE_URL" ]; then
    log "download nginx template from ${NGINX_TEMPLATE_URL}"
    download "$NGINX_TEMPLATE_URL" "$conf_tmp"
  else
    fetch_url="https://raw.githubusercontent.com/${REPO}/${release_tag}/install/nginx.conf"
    fallback_url="https://raw.githubusercontent.com/${REPO}/main/install/nginx.conf"
    log "download nginx template from ${fetch_url}"
    if ! download "$fetch_url" "$conf_tmp"; then
      warn "cannot download nginx template from release tag, fallback to main"
      download "$fallback_url" "$conf_tmp"
    fi
  fi

  [ -s "$conf_tmp" ] || die "downloaded nginx template is empty"
  NGINX_TEMPLATE_FILE="$conf_tmp"
}

install_nginx_reverse_proxy() {
  local release_tag="$1"
  local app_host proxy_server_name backend_port conf_tmp
  ensure_nginx_installed
  ensure_nginx_cache_dir
  ensure_nginx_template_file "$release_tag"

  app_host="$(read_env_value "APP_HOSTNAME" "$ENV_FILE")"
  [ -n "$app_host" ] || app_host="aurora-admin.local"
  proxy_server_name="$app_host"

  backend_port="$BACKEND_PORT"
  if [ -z "$backend_port" ]; then
    backend_port="$(read_env_value "APP_PORT" "$ENV_FILE")"
  fi
  [ -n "$backend_port" ] || die "cannot resolve backend APP_PORT for nginx"
  [ -f "$NGINX_TEMPLATE_FILE" ] || die "nginx template file not found: ${NGINX_TEMPLATE_FILE}"

  conf_tmp="${TMP_DIR}/aurora-admin-nginx.conf"
  render_nginx_template "$NGINX_TEMPLATE_FILE" "$conf_tmp" "$proxy_server_name" "$backend_port"

  as_root install -m 0644 -o root -g root "$conf_tmp" "$NGINX_CONF_FILE"
  migrate_nginx_http2_deprecated_all
  as_root nginx -t
  as_root systemctl daemon-reload
  as_root systemctl enable "$NGINX_SERVICE_NAME"
  as_root systemctl restart "$NGINX_SERVICE_NAME"
  log "nginx reverse proxy ready: https://${proxy_server_name} -> 127.0.0.1:${backend_port}"
  log "nginx template: ${NGINX_TEMPLATE_FILE}"
  log "nginx cache dir: ${NGINX_CACHE_DIR}"
}

install_systemd_service() {
  local tag="$1"
  local service_url="https://raw.githubusercontent.com/${REPO}/${tag}/install/aurora-admin.service"
  local source_service="${TMP_DIR}/aurora-admin.service"

  log "download service template (${REPO}@${tag})"
  download "$service_url" "$source_service"

  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$source_service" "$SERVICE_PATH"
  as_root systemctl daemon-reload
  as_root systemctl enable "$SERVICE_NAME"
}

restart_service() {
  log "restart ${SERVICE_NAME}"
  as_root systemctl restart "$SERVICE_NAME"
}

delete_source_env_if_needed() {
  local source_env="$1"
  if [ "$DELETE_INPUT_ENV" != "true" ]; then
    return
  fi

  local src_abs env_abs
  src_abs="$(readlink -f "$source_env" 2>/dev/null || printf '%s' "$source_env")"
  env_abs="$(readlink -f "$ENV_FILE" 2>/dev/null || printf '%s' "$ENV_FILE")"
  if [ "$src_abs" = "$env_abs" ]; then
    return
  fi

  rm -f "$source_env" || warn "cannot delete source env file: ${source_env}"
}

main() {
  trap cleanup EXIT
  parse_args "$@"

  if [ "$MODE" = "config" ]; then
    write_config_template "$CONFIG_OUTPUT"
    log "generated config template: ${CONFIG_OUTPUT}"
    log "next: ./install.sh -f ${CONFIG_OUTPUT}"
    exit 0
  fi

  [ -n "$INPUT_ENV_FILE" ] || die "missing env file. Use: ./install.sh -f <env_file>"

  require_cmd tar
  require_cmd sha256sum
  require_cmd systemctl
  require_cmd openssl

  TMP_DIR="$(mktemp -d)"
  local machine_arch tag
  machine_arch="$(arch)"
  tag="$(resolve_tag)"
  [ -n "$tag" ] || die "cannot resolve release tag"

  ensure_service_user
  ensure_service_user_sudo_group
  ensure_tls_dir
  install_binary "$tag" "$machine_arch"
  install_env_file "$INPUT_ENV_FILE"
  assign_random_backend_port
  install_tls_materials "$ENV_FILE"
  preflight_tls_materials
  install_systemd_service "$tag"
  install_nginx_reverse_proxy "$tag"
  restart_service
  delete_source_env_if_needed "$INPUT_ENV_FILE"

  log "done"
  log "binary: ${BIN_PATH}"
  log "service: ${SERVICE_NAME}"
  log "nginx: ${NGINX_SERVICE_NAME}"
  log "nginx config: ${NGINX_CONF_FILE}"
  log "backend app_port: ${BACKEND_PORT}"
  log "env: ${ENV_FILE}"
  log "ui: embedded in binary"
  log "release: ${REPO}@${tag}"
  log "check: sudo systemctl status ${SERVICE_NAME} --no-pager"
}

main "$@"
