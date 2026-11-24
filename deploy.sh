#!/usr/bin/env bash
set -e

# ==============================================================================
# JobyUK.com Multi-Tenant Production 1-Click Deployment (Ubuntu / Debian VPS)
# 100% Non-Destructive: Safe alongside Jobies, AnimeWatch (3000), iLookio & Existing Projects
# Uses Dedicated Isolated Port: 8082
# ==============================================================================

echo "=================================================="
echo "🚀 Starting 1-Click JobyUK.com VPS Production Deploy"
echo "=================================================="

APP_DIR="/var/www/jobyuk"
SERVICE_NAME="jobyuk"
PORT=8082

# 0. Safety Port Conflict Check (Protects Jobies, AnimeWatch, iLookio)
echo "🔍 Checking for port ${PORT} conflicts..."
if command -v ss &> /dev/null; then
    CONFLICT_LINE=$(sudo ss -tulpn | grep ":${PORT} " || true)
    if [ -n "$CONFLICT_LINE" ]; then
        ACTIVE_PID=$(echo "$CONFLICT_LINE" | grep -o 'pid=[0-9]*' | cut -d'=' -f2 | head -n1 || true)
        PROC_NAME=""
        if [ -n "$ACTIVE_PID" ]; then
            PROC_NAME=$(ps -p "$ACTIVE_PID" -o comm= 2>/dev/null || true)
        fi
        if [ "$PROC_NAME" != "jobyuk" ] && [ -n "$PROC_NAME" ]; then
            echo "❌ PORT OVERLAP ALERT: Port ${PORT} is currently in use by '$PROC_NAME' (PID $ACTIVE_PID)!"
            echo "Existing process details: $CONFLICT_LINE"
            echo "🚫 Deployment stopped to protect existing services. Choose an alternate port (e.g. 8085)."
            exit 1
        else
            echo "ℹ️ Port ${PORT} is held by previous jobyuk instance. Safe to update."
        fi
    else
        echo "✅ Port ${PORT} is completely FREE and isolated."
    fi
fi

# 1. Ensure Directory Structure
if [ ! -d "$APP_DIR" ]; then
    echo "📁 Creating application directory: $APP_DIR"
    sudo mkdir -p "$APP_DIR"
    sudo chown -R $USER:$USER "$APP_DIR" 2>/dev/null || true
fi

cd "$APP_DIR"

# 2. Check and Install Go Compiler if needed
export PATH=/usr/local/go/bin:$PATH:$HOME/go/bin
if ! command -v go &> /dev/null; then
    echo "🐹 Go not found. Installing Golang 1.23.2..."
    sudo apt-get update -y
    sudo apt-get install -y curl wget git build-essential
    wget -q https://go.dev/dl/go1.23.2.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.2.linux-amd64.tar.gz
    rm -f go1.23.2.linux-amd64.tar.gz
    export PATH=/usr/local/go/bin:$PATH
fi

echo "🐹 Go Compiler Ready: $(go version)"

# 3. Pull latest code if this is a Git repo
if [ -d ".git" ]; then
    echo "📥 Pulling latest updates from GitHub..."
    git pull origin main 2>/dev/null || true
fi

# 4. Check PostgreSQL Database
echo "🗄️ Checking PostgreSQL database jobyuk_db..."
if command -v psql &> /dev/null; then
    sudo -u postgres psql -tc "SELECT 1 FROM pg_database WHERE datname = 'jobyuk_db'" | grep -q 1 || \
    sudo -u postgres psql -c "CREATE DATABASE jobyuk_db;"
    echo "✅ Database jobyuk_db is ready."
fi

# 5. Ensure .env exists
if [ ! -f ".env" ]; then
    echo "⚙️ Creating default production .env..."
    cat << 'EOF' > .env
PORT=8082
BASE_URL=https://jobyuk.com
ENV=production
DATABASE_URL=postgres://postgres:postgres@localhost:5432/jobyuk_db?sslmode=disable
REDIS_URL=redis://localhost:6379/0
REED_API_KEY=your_reed_api_key_here
JWT_SECRET=jobyuk-super-secret-production-jwt-key-32chars
CSRF_SECRET=jobyuk-super-secret-production-csrf-key-32chars
ADMIN_USERNAME=admin
ADMIN_PASSWORD=change_this_admin_password_123
INDEXNOW_KEY=ab4225315c73f110cb98d2abd9911d76
ENABLE_WHATSAPP_ALERTS=false
EOF
    echo "⚠️ Created /var/www/jobyuk/.env - Please verify database password and secrets."
fi

# 6. Compile High-Performance Go Binary
echo "🔨 Compiling production binary (jobyuk)..."
go build -ldflags="-s -w" -o jobyuk.tmp ./cmd/server
mv -f jobyuk.tmp jobyuk
chmod +x jobyuk

# 7. Create Dedicated Isolated Systemd Service
echo "⚙️ Setting up /etc/systemd/system/${SERVICE_NAME}.service..."
sudo bash -c "cat << 'EOF' > /etc/systemd/system/${SERVICE_NAME}.service
[Unit]
Description=JobyUK.com Production Web Application
After=network.target postgresql.service redis-server.service

[Service]
Type=simple
User=root
WorkingDirectory=${APP_DIR}
ExecStart=${APP_DIR}/jobyuk
Restart=always
RestartSec=3s
LimitNOFILE=65535
EnvironmentFile=${APP_DIR}/.env
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF"

# 8. Reload Systemd & Start Service
sudo systemctl daemon-reload
sudo systemctl enable "${SERVICE_NAME}"
sudo systemctl restart "${SERVICE_NAME}"

# 9. Setup Nginx Reverse Proxy Configuration
if command -v nginx &> /dev/null; then
    echo "🌐 Configuring Nginx for jobyuk.com & www.jobyuk.com..."
    sudo bash -c "cat << 'EOF' > /etc/nginx/sites-available/jobyuk
server {
    listen 80;
    listen [::]:80;
    server_name jobyuk.com www.jobyuk.com;

    client_max_body_size 50M;

    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_proxied any;
    gzip_types text/plain text/css text/xml text/javascript application/javascript application/json font/woff2 font/woff;

    # Static Assets Direct Caching
    location /static/ {
        alias ${APP_DIR}/web/static/;
        expires 30d;
        add_header Cache-Control \"public, no-transform\";
        access_log off;
    }

    location / {
        proxy_pass http://127.0.0.1:${PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_set_header CF-Connecting-IP \$http_cf_connecting_ip;
        proxy_set_header True-Client-IP \$http_true_client_ip;

        proxy_connect_timeout 60s;
        proxy_send_timeout 120s;
        proxy_read_timeout 120s;
    }
}
EOF"

    sudo ln -sf /etc/nginx/sites-available/jobyuk /etc/nginx/sites-enabled/
    sudo nginx -t 2>/dev/null && sudo systemctl reload nginx && echo "✅ Nginx reloaded successfully!" || echo "⚠️ Nginx test failed, check configs."
fi

# 10. Open Port in UFW if firewall is active
sudo ufw allow ${PORT}/tcp 2>/dev/null || true

echo "=================================================="
echo "🎉 DEPLOYMENT COMPLETE! Status of ${SERVICE_NAME}:"
sudo systemctl status "${SERVICE_NAME}" --no-pager
echo "=================================================="
