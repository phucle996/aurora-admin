#!/usr/bin/env bash
set -Eeuo pipefail

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

CONFIG_OUTPUT="${AURORA_ADMIN_CONFIG_OUTPUT:-./aurora-admin.env.sample}"
INPUT_ENV_FILE=""
MODE="install"
TMP_DIR=""
DELETE_INPUT_ENV="true"

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
  line="$(grep -E "^${key}=" "$file" | tail -n1 || true)"
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

write_config_template() {
  local out="$1"
  cat > "$out" <<'ENV_TPL'
# Aurora Admin Service env template
# Fill all required values before install.

APP_HOSTNAME=aurora-admin.local
APP_PORT=3009
APP_LOG_LEVEL=info
APP_TIMEZONE=Asia/Ho_Chi_Minh

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
  app_host="$(read_env_value "APP_HOSTNAME" "$env_file")"
  [ -n "$app_host" ] || app_host="aurora-admin.local"

  cert_tmp="${TMP_DIR}/admin.crt"
  key_tmp="${TMP_DIR}/admin.key"
  ca_tmp="${TMP_DIR}/ca.crt"
  ca_key_tmp="${TMP_DIR}/ca.key"
  csr_tmp="${TMP_DIR}/admin.csr"
  ext_tmp="${TMP_DIR}/admin.ext"

  log "generate self-signed tls cert/key/ca"
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

  as_root mkdir -p "$TLS_DIR"
  as_root chown root:"$SERVICE_GROUP" "$TLS_DIR"
  as_root chmod 750 "$TLS_DIR"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$cert_tmp" "$TLS_CERT_FILE"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$key_tmp" "$TLS_KEY_FILE"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$ca_tmp" "$TLS_CA_FILE"
  as_root install -m 0400 -o "$SERVICE_USER" -g "$SERVICE_GROUP" "$ca_key_tmp" "$TLS_CA_KEY_FILE"
}

preflight_tls_materials() {
  log "preflight tls materials"
  for path in "$TLS_CERT_FILE" "$TLS_KEY_FILE" "$TLS_CA_FILE" "$TLS_CA_KEY_FILE"; do
    as_root test -s "$path" || die "tls preflight failed: missing file $path"
  done

  local cert_check key_check ca_check verify_check cert_mod key_mod
  cert_check="$(as_root openssl x509 -in "$TLS_CERT_FILE" -noout 2>&1)" || die "tls preflight failed: invalid cert ${TLS_CERT_FILE}: ${cert_check}"
  key_check="$(as_root openssl rsa -in "$TLS_KEY_FILE" -check -noout 2>&1)" || die "tls preflight failed: invalid key ${TLS_KEY_FILE}: ${key_check}"
  ca_check="$(as_root openssl x509 -in "$TLS_CA_FILE" -noout 2>&1)" || die "tls preflight failed: invalid ca ${TLS_CA_FILE}: ${ca_check}"

  verify_check="$(as_root openssl verify -CAfile "$TLS_CA_FILE" "$TLS_CERT_FILE" 2>&1)" || die "tls preflight failed: cert is not signed by ca: ${verify_check}"
  cert_mod="$(as_root openssl x509 -noout -modulus -in "$TLS_CERT_FILE" | openssl md5 2>/dev/null | awk '{print $NF}')"
  key_mod="$(as_root openssl rsa -noout -modulus -in "$TLS_KEY_FILE" | openssl md5 2>/dev/null | awk '{print $NF}')"
  [ -n "$cert_mod" ] || die "tls preflight failed: cannot read cert modulus"
  [ -n "$key_mod" ] || die "tls preflight failed: cannot read key modulus"
  [ "$cert_mod" = "$key_mod" ] || die "tls preflight failed: cert/key mismatch"
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
  install_binary "$tag" "$machine_arch"
  install_env_file "$INPUT_ENV_FILE"
  install_tls_materials "$ENV_FILE"
  preflight_tls_materials
  install_systemd_service "$tag"
  restart_service
  delete_source_env_if_needed "$INPUT_ENV_FILE"

  log "done"
  log "binary: ${BIN_PATH}"
  log "service: ${SERVICE_NAME}"
  log "env: ${ENV_FILE}"
  log "ui: embedded in binary"
  log "release: ${REPO}@${tag}"
  log "check: sudo systemctl status ${SERVICE_NAME} --no-pager"
}

main "$@"
