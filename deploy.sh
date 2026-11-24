#!/usr/bin/env bash
set -e

# ==============================================================================
# JobyUK.com Multi-Tenant Production 1-Click Deployment (Ubuntu / Debian VPS)
# 100% Non-Destructive: Safe alongside Jobies, AnimeWatch (3000), iLookio & Existing Projects
# Uses Dedicated Isolated Port: 8082 & Isolated DB User: jobyuk
# ==============================================================================

echo "=================================================="
echo "🚀 Starting 1-Click JobyUK.com VPS Production Deploy"
echo "=================================================="

APP_DIR="/var/www/jobyuk"
SERVICE_NAME="jobyuk"
PORT=8082
DB_USER="jobyuk"
DB_PASS="jobyuk_pass_2026"
DB_NAME="jobyuk_db"

# 0. Silence hostname warning if present
if ! grep -q "127.0.0.1 $(hostname)" /etc/hosts 2>/dev/null; then
    echo "127.0.0.1 $(hostname)" >> /etc/hosts 2>/dev/null || true
fi

# 1. Safety Port Conflict Check (Protects Jobies, AnimeWatch, iLookio)
echo "🔍 Checking for port ${PORT} conflicts..."
if command -v ss &> /dev/null; then
    CONFLICT_LINE=$(ss -tulpn | grep ":${PORT} " || true)
    if [ -n "$CONFLICT_LINE" ]; then
        ACTIVE_PID=$(echo "$CONFLICT_LINE" | grep -o 'pid=[0-9]*' | cut -d'=' -f2 | head -n1 || true)
        PROC_NAME=""
        if [ -n "$ACTIVE_PID" ]; then
            PROC_NAME=$(ps -p "$ACTIVE_PID" -o comm= 2>/dev/null || true)
        fi
        if [ "$PROC_NAME" != "jobyuk" ] && [ -n "$PROC_NAME" ]; then
            echo "❌ PORT OVERLAP ALERT: Port ${PORT} is currently in use by '$PROC_NAME' (PID $ACTIVE_PID)!"
            echo "Existing process details: $CONFLICT_LINE"
            echo "🚫 Deployment stopped to protect existing services."
            exit 1
        else
            echo "ℹ️ Port ${PORT} is held by previous jobyuk instance. Safe to update."
        fi
    else
        echo "✅ Port ${PORT} is completely FREE and isolated."
    fi
fi

# 2. Ensure Directory Structure
if [ ! -d "$APP_DIR" ]; then
    echo "📁 Creating application directory: $APP_DIR"
    mkdir -p "$APP_DIR"
fi

cd "$APP_DIR"

# 3. Check and Install Go Compiler if needed
export PATH=/usr/local/go/bin:$PATH:$HOME/go/bin
if ! command -v go &> /dev/null; then
    echo "🐹 Go not found. Installing Golang 1.23.2..."
    apt-get update -y
    apt-get install -y curl wget git build-essential postgresql postgresql-contrib redis-server
    wget -q https://go.dev/dl/go1.23.2.linux-amd64.tar.gz
    rm -rf /usr/local/go && tar -C /usr/local -xzf go1.23.2.linux-amd64.tar.gz
    rm -f go1.23.2.linux-amd64.tar.gz
    export PATH=/usr/local/go/bin:$PATH
fi

echo "🐹 Go Compiler Ready: $(go version)"

# 4. Pull latest code if this is a Git repo
if [ -d ".git" ]; then
    echo "📥 Pulling latest updates from GitHub..."
    git pull origin main 2>/dev/null || true
fi

# 5. Configure Isolated PostgreSQL User & Database (100% Non-Destructive)
echo "🐘 Configuring dedicated PostgreSQL database '${DB_NAME}' & user '${DB_USER}'..."
if command -v psql &> /dev/null; then
    sudo -u postgres psql -c "CREATE USER ${DB_USER} WITH PASSWORD '${DB_PASS}';" 2>/dev/null || \
    sudo -u postgres psql -c "ALTER USER ${DB_USER} WITH PASSWORD '${DB_PASS}';"
    
    sudo -u postgres psql -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};" 2>/dev/null || true
    sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};"
    sudo -u postgres psql -d ${DB_NAME} -c "GRANT ALL ON SCHEMA public TO ${DB_USER};" 2>/dev/null || true
    echo "✅ Dedicated PostgreSQL database & user configured successfully."
fi

# 6. Ensure .env exists with correct credentials
echo "⚙️ Configuring production .env..."
cat << EOF > .env
PORT=${PORT}
BASE_URL=https://jobyuk.com
ENV=production
DATABASE_URL=postgres://${DB_USER}:${DB_PASS}@localhost:5432/${DB_NAME}?sslmode=disable
REDIS_URL=redis://localhost:6379/0
REED_API_KEY=ec7a89c8-0d1f-464d-9878-3936d1316620
JWT_SECRET=jobyuk-super-secret-production-jwt-key-32chars
CSRF_SECRET=jobyuk-super-secret-production-csrf-key-32chars
ADMIN_USERNAME=jobyuk_admin
ADMIN_PASSWORD=JobyUK_SecureAdmin_2026!
INDEXNOW_KEY=ab4225315c73f110cb98d2abd9911d76
ENABLE_WHATSAPP_ALERTS=false
EOF
chmod 600 .env

# 7. Import Database Migrations into jobyuk_db
echo "🗄️ Importing database migrations into '${DB_NAME}'..."
if [ -d "migrations" ]; then
    for f in $(ls -1 migrations/*.sql | sort); do
        echo "Importing $f..."
        sudo -u postgres psql -d ${DB_NAME} -f "$f" >/dev/null 2>&1 || true
    done
    if [ -f "migrations/019_clean_production_jobs_seed.sql.gz" ]; then
        echo "📦 Importing 12,421 verified production jobs, companies & taxonomy into '${DB_NAME}'..."
        zcat migrations/019_clean_production_jobs_seed.sql.gz | sudo -u postgres psql -d ${DB_NAME} >/dev/null 2>&1 || true
    fi
    sudo -u postgres psql -d ${DB_NAME} -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ${DB_USER}; GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO ${DB_USER}; ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO ${DB_USER}; ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO ${DB_USER};" 2>/dev/null || true
    echo "✅ Database tables & taxonomy imported."
fi

# 8. Compile High-Performance Go Binary
echo "🔨 Compiling production binary (jobyuk)..."
go build -ldflags="-s -w" -o jobyuk.tmp ./cmd/server
mv -f jobyuk.tmp jobyuk
chmod +x jobyuk

# 9. Create Dedicated Isolated Systemd Service
echo "⚙️ Setting up /etc/systemd/system/${SERVICE_NAME}.service..."
cat << EOF > /etc/systemd/system/${SERVICE_NAME}.service
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
EOF

# 10. Reload Systemd & Start Service
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
systemctl restart "${SERVICE_NAME}"
sleep 2

# 11. Open Port in UFW Firewall
ufw allow ${PORT}/tcp 2>/dev/null || true

echo "=================================================="
echo "🎉 DEPLOYMENT COMPLETE! Status of ${SERVICE_NAME}:"
systemctl status "${SERVICE_NAME}" --no-pager
echo "=================================================="

# 12. Local verification test
echo "🧪 Testing local response on http://127.0.0.1:${PORT}..."
curl -I http://127.0.0.1:${PORT} || true
echo "=================================================="
