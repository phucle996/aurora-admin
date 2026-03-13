#!/usr/bin/env bash

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
