#!/usr/bin/env sh
set -eu

CERT_DIR="${AURORA_CERT_DIR:-/etc/aurora/certs}"
mkdir -p "${CERT_DIR}"

CA_KEY="${CERT_DIR}/ca.key"
CA_CRT="${CERT_DIR}/ca.crt"

ADMIN_KEY="${CERT_DIR}/admin.key"
ADMIN_CRT="${CERT_DIR}/admin.crt"

NGINX_CLIENT_KEY="${CERT_DIR}/nginx-client.key"
NGINX_CLIENT_CRT="${CERT_DIR}/nginx-client.crt"

if [ ! -s "${CA_KEY}" ] || [ ! -s "${CA_CRT}" ]; then
  echo "[cert] generating dev CA"
  openssl genrsa -out "${CA_KEY}" 4096 >/dev/null 2>&1
  openssl req -x509 -new -nodes -key "${CA_KEY}" -sha256 -days 3650 \
    -out "${CA_CRT}" -subj "/CN=Aurora Dev Root CA" >/dev/null 2>&1
fi

if [ ! -s "${ADMIN_KEY}" ] || [ ! -s "${ADMIN_CRT}" ]; then
  echo "[cert] generating admin server cert"
  openssl genrsa -out "${ADMIN_KEY}" 2048 >/dev/null 2>&1
  openssl req -new -key "${ADMIN_KEY}" -out "${CERT_DIR}/admin.csr" \
    -subj "/CN=admin-service" >/dev/null 2>&1
  cat > "${CERT_DIR}/admin.ext" <<'EOF'
subjectAltName=DNS:admin-service,DNS:admin.aurora.local,DNS:localhost,IP:127.0.0.1
extendedKeyUsage=serverAuth
keyUsage=digitalSignature,keyEncipherment
EOF
  openssl x509 -req -in "${CERT_DIR}/admin.csr" -CA "${CA_CRT}" -CAkey "${CA_KEY}" \
    -CAcreateserial -out "${ADMIN_CRT}" -days 825 -sha256 -extfile "${CERT_DIR}/admin.ext" >/dev/null 2>&1
fi

if [ ! -s "${NGINX_CLIENT_KEY}" ] || [ ! -s "${NGINX_CLIENT_CRT}" ]; then
  echo "[cert] generating nginx client cert"
  openssl genrsa -out "${NGINX_CLIENT_KEY}" 2048 >/dev/null 2>&1
  openssl req -new -key "${NGINX_CLIENT_KEY}" -out "${CERT_DIR}/nginx-client.csr" \
    -subj "/CN=aurora-nginx" >/dev/null 2>&1
  cat > "${CERT_DIR}/nginx-client.ext" <<'EOF'
extendedKeyUsage=clientAuth
keyUsage=digitalSignature,keyEncipherment
subjectAltName=DNS:aurora-nginx,DNS:nginx
EOF
  openssl x509 -req -in "${CERT_DIR}/nginx-client.csr" -CA "${CA_CRT}" -CAkey "${CA_KEY}" \
    -CAcreateserial -out "${NGINX_CLIENT_CRT}" -days 825 -sha256 -extfile "${CERT_DIR}/nginx-client.ext" >/dev/null 2>&1
fi

rm -f "${CERT_DIR}/admin.csr" "${CERT_DIR}/admin.ext" \
  "${CERT_DIR}/nginx-client.csr" "${CERT_DIR}/nginx-client.ext" \
  "${CERT_DIR}/ca.srl"

chmod 0400 "${CA_KEY}" "${ADMIN_KEY}" || true
chmod 0444 "${NGINX_CLIENT_KEY}" || true
chmod 0444 "${CA_CRT}" "${ADMIN_CRT}" "${NGINX_CLIENT_CRT}" || true

echo "[cert] ready in ${CERT_DIR}"
