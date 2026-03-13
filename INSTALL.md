# Install Guide

## 1) Install from GitHub (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/phucle996/aurora-admin/main/install/install.sh -o install.sh
chmod +x install.sh
```

Generate env template:

```bash
./install.sh --config ./aurora-admin.env
```

Edit `./aurora-admin.env`, then install:

```bash
sudo ./install.sh -f ./aurora-admin.env
```

Install a specific release:

```bash
sudo AURORA_ADMIN_VERSION=v20260305120000-abc12345-alpha ./install.sh -f ./aurora-admin.env
```

## 2) Verify service

```bash
sudo systemctl status aurora-admin.service --no-pager
sudo journalctl -u aurora-admin.service -n 100 --no-pager
```

## 3) Uninstall (manual)

```bash
sudo systemctl stop aurora-admin.service || true
sudo systemctl disable aurora-admin.service || true
sudo rm -f /etc/systemd/system/aurora-admin.service
sudo systemctl daemon-reload

sudo rm -f /usr/local/bin/aurora-admin-service
sudo rm -f /etc/nginx/conf.d/aurora-admin.conf
sudo systemctl restart nginx || true
```

## Notes

- Script auto-generates TLS materials under `/etc/aurora/certs` if missing.
- Service runs under non-root user `aurora`.
- Nginx reverse proxy is installed/configured by installer.
