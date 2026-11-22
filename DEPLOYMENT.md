# 🚀 JobyUK Production Deployment Guide

A comprehensive, step-by-step production deployment and maintenance manual for **JobyUK** ([jobyuk.com](https://jobyuk.com)).

---

## 📌 1. Architecture Overview

- **Web Application Backend**: High-performance Go (Golang 1.22+) compiled binary.
- **Database**: PostgreSQL 16+ (Connection pooling, indexing, CASCADE constraints).
- **Cache & Rate Limiting**: Redis 7+ (Search cache, rate limiting, and session tracking).
- **Frontend / Assets**: Server-rendered HTML templates with minified CSS and JavaScript bundles.
- **Reverse Proxy & SSL**: Nginx / Caddy reverse proxy on port `80`/`443` proxying to Go on `127.0.0.1:8080`.
- **CDN & Edge Security**: Cloudflare (SSL Full/Strict, DDoS mitigation, CF-Connecting-IP forwarding).
- **CI/CD Pipeline**: GitHub Actions Self-Hosted Runner on VPS for zero-downtime auto-deploy on `git push origin main`.

---

## 🏗️ 2. VPS Prerequisites & Initial Setup

Install the required system packages on Ubuntu / Debian:

```bash
# 1. Update system packages
sudo apt update && sudo apt upgrade -y

# 2. Install essential build tools & Git
sudo apt install -y curl wget git build-essential ufw

# 3. Install Golang (1.22+)
wget https://go.dev/dl/go1.22.4.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.22.4.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 4. Install PostgreSQL 16
sudo apt install -y postgresql postgresql-contrib

# 5. Install Redis Server
sudo apt install -y redis-server
sudo systemctl enable redis-server
sudo systemctl start redis-server

# 6. Install Nginx
sudo apt install -y nginx
sudo systemctl enable nginx
sudo systemctl start nginx
```

---

## 🗄️ 3. PostgreSQL Database Setup

Create the database and user:

```bash
sudo -u postgres psql
```

Inside PostgreSQL prompt:

```sql
CREATE DATABASE JobyUK;
CREATE USER JobyUK WITH ENCRYPTED PASSWORD 'your_strong_db_password_here';
GRANT ALL PRIVILEGES ON DATABASE JobyUK TO JobyUK;
ALTER DATABASE JobyUK OWNER TO JobyUK;
\q
```

---

## 📁 4. Application Directory Structure on VPS

Create the deployment directory:

```bash
sudo mkdir -p /var/www/JobyUK
sudo chown -R root:root /var/www/JobyUK
sudo chmod -R 755 /var/www/JobyUK
```

Expected directory layout inside `/var/www/JobyUK`:

```text
/var/www/JobyUK/
├── JobyUK                  # Compiled Go binary
├── .env                      # Production environment configuration
├── google-credentials.json   # Google Indexing Service Account JSON (optional)
├── data/
│   └── site_settings.json    # Dynamic site & CTA settings
├── web/
│   ├── templates/            # HTML templates
│   └── static/               # CSS, JS, Images, Icons
└── migrations/               # SQL schema migrations
```

---

## ⚙️ 5. Production `.env` File Setup

Create `/var/www/JobyUK/.env`:

```bash
sudo nano /var/www/JobyUK/.env
```

Paste your production variables:

```ini
# Server Configuration
PORT=8080
BASE_URL=https://jobyuk.com
ENV=production

# Database Configuration
DATABASE_URL=postgres://JobyUK:your_strong_db_password_here@localhost:5432/JobyUK?sslmode=disable

# Redis Configuration
REDIS_URL=redis://localhost:6379/0

# Reed API Integration (Optional)
REED_API_KEY=your_reed_api_key_here

# Security Secrets (Generate random 32+ char strings)
JWT_SECRET=your_super_secret_jwt_key_here
CSRF_SECRET=your_super_secret_csrf_key_here

# Admin Login Credentials
ADMIN_USERNAME=admin
ADMIN_PASSWORD=your_secure_admin_password
```

Secure the `.env` file:

```bash
sudo chmod 600 /var/www/JobyUK/.env
```

---

## 🚀 6. Systemd Service Configuration

Create the systemd service file:

```bash
sudo nano /etc/systemd/system/JobyUK.service
```

Paste the following configuration:

```ini
[Unit]
Description=JobyUK Production Web Application
After=network.target postgresql.service redis-server.service

[Service]
Type=simple
User=root
WorkingDirectory=/var/www/JobyUK
ExecStart=/var/www/JobyUK/JobyUK
Restart=always
RestartSec=3s
LimitNOFILE=65535
EnvironmentFile=/var/www/JobyUK/.env

# Standard output and error logging
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

Reload and enable the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable JobyUK
sudo systemctl start JobyUK
```

---

## 🌐 7. Nginx Reverse Proxy Configuration

Create `/etc/nginx/sites-available/JobyUK`:

```bash
sudo nano /etc/nginx/sites-available/JobyUK
```

Paste the Nginx configuration:

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name jobyuk.com www.jobyuk.com;

    # Client body size limit for uploads (e.g. indexing json or logos)
    client_max_body_size 20M;

    # Gzip Compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_proxied expired no-cache no-store private auth;
    gzip_types text/plain text/css text/xml text/javascript application/javascript application/x-javascript application/xml application/json font/woff2 font/woff;

    # Static Assets Direct Caching
    location /static/ {
        alias /var/www/JobyUK/web/static/;
        expires 30d;
        add_header Cache-Control "public, no-transform";
        access_log off;
    }

    # Proxy all traffic to Go application
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Pass Cloudflare headers
        proxy_set_header CF-Connecting-IP $http_cf_connecting_ip;
        proxy_set_header True-Client-IP $http_true_client_ip;

        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
```

Enable site and test configuration:

```bash
sudo ln -sf /etc/nginx/sites-available/JobyUK /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl reload nginx
```

---

## 🔄 8. Automated CI/CD Deployment (GitHub Actions)

Deployments are fully automated via GitHub Actions on every `git push origin main`.

The workflow file is located at:
[`.github/workflows/deploy.yml`](file:///.github/workflows/deploy.yml)

### Workflow Steps:
1. **Trigger**: Push to branch `main`.
2. **Runner**: Self-hosted runner installed on your VPS.
3. **Build**: Compiles Go binary (`go build -o JobyUK ./cmd/server`).
4. **Deploy**:
   - Stops the systemd service gracefully (`systemctl stop JobyUK`).
   - Copies the compiled binary, templates (`web/`), and migrations (`migrations/`) into `/var/www/JobyUK`.
   - Restarts the service (`systemctl start JobyUK`).

### Setting up the Self-Hosted Runner (If reinstalling):
1. Go to your GitHub Repository ➔ **Settings** ➔ **Actions** ➔ **Runners** ➔ **New self-hosted runner**.
2. Run the provided download and configure scripts on the VPS as `root`.
3. Install and start as a service:
   ```bash
   sudo ./svc.sh install
   sudo ./svc.sh start
   ```

---

## 🛠️ 9. Essential VPS Maintenance Commands

### Service Commands:
```bash
# Check status of the Go application
sudo systemctl status JobyUK

# Restart the application
sudo systemctl restart JobyUK

# Stop the application
sudo systemctl stop JobyUK

# Start the application
sudo systemctl start JobyUK
```

### Real-Time Log Monitoring:
```bash
# View live application logs
sudo journalctl -u JobyUK -f

# View last 100 log lines
sudo journalctl -u JobyUK -n 100 --no-pager

# View Nginx access & error logs
sudo tail -f /var/log/nginx/access.log
sudo tail -f /var/log/nginx/error.log
```

### Database & Redis Checks:
```bash
# Connect to PostgreSQL
sudo -u postgres psql -d JobyUK

# Check Redis ping
redis-cli ping   # Should output: PONG

# Clear Redis cache manually
redis-cli FLUSHDB
```

---

## 🩺 10. Troubleshooting Guide

| Issue | Cause | Solution |
| :--- | :--- | :--- |
| **502 Bad Gateway** | Go application is restarting or down | Run `systemctl status JobyUK` and check `journalctl -u JobyUK -n 50` for crash errors. |
| **Port 8080 already in use** | Stray Go process running in background | Run `pkill JobyUK` or `fuser -k 8080/tcp`, then `systemctl restart JobyUK`. |
| **CSS / JS not updating** | Browser or CDN caching old assets | Clear Cloudflare cache or hard refresh browser with `Ctrl + Shift + R` (`Cmd + Shift + R`). |
| **Google Indexing API Error** | Service account JSON missing or invalid permissions | Upload your Service Account JSON in Admin Settings (`/admin-lol/settings`) or paste it directly. |
| **Database connection refused** | PostgreSQL service stopped | Run `sudo systemctl restart postgresql` and verify `DATABASE_URL` in `.env`. |

---

✅ **Your application is now configured for high reliability, automatic background updates, and zero manual deployment overhead!**
