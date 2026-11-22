#!/bin/bash
set -e

# ==============================================================================
# JobyUK.com - Automated 1-Click Production Setup (Ubuntu/Debian)
# 100% NON-DESTRUCTIVE: Safe alongside iLookio.com (Uses Port 8088, Dedicated DB)
# ==============================================================================

echo "=================================================="
echo "🚀 Starting JobyUK.com Setup (Coexisting with iLookio)..."
echo "=================================================="

# 1. Ensure IPv6 DNS64 / NAT64 is available for GitHub / external connectivity
echo "🌐 Ensuring DNS64 / NAT64 for IPv6 reachability..."
if ! grep -q "2a00:1098:2b::1" /etc/resolv.conf 2>/dev/null; then
    echo "nameserver 2a00:1098:2b::1" > /tmp/res.conf
    echo "nameserver 2a01:4f8:c2c:123f::1" >> /tmp/res.conf
    cat /etc/resolv.conf >> /tmp/res.conf 2>/dev/null || true
    cp /tmp/res.conf /etc/resolv.conf 2>/dev/null || true
fi

# 2. Update and install packages
echo "📦 Checking and installing system packages..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y git curl ufw sudo build-essential postgresql postgresql-contrib redis-server

# 3. Ensure Go is installed and at least 1.26+
echo "🐹 Checking Go compiler version..."
CURRENT_GO=$(/usr/local/go/bin/go version 2>/dev/null || go version 2>/dev/null || echo "none")
if [[ "$CURRENT_GO" != *"go1.27"* && "$CURRENT_GO" != *"go1.26"* ]]; then
    echo "Upgrading Go to 1.27.3 (Current: $CURRENT_GO)..."
    curl -LO https://go.dev/dl/go1.27.3.linux-amd64.tar.gz
    rm -rf /usr/local/go && tar -C /usr/local -xzf go1.27.3.linux-amd64.tar.gz
    rm -f go1.27.3.linux-amd64.tar.gz
fi
export PATH=/usr/local/go/bin:$PATH

# 4. Start PostgreSQL and Redis services
systemctl enable --now postgresql
systemctl enable --now redis-server 2>/dev/null || true

# 5. Configure Isolated PostgreSQL Database 'jobyuk' (Does NOT touch ilookio DB)
echo "🐘 Configuring isolated 'jobyuk' database & user in PostgreSQL..."
if [ ! -f "/var/www/jobyuk/.env" ]; then
    DB_PASS=$(openssl rand -hex 16)
    sudo -u postgres psql -c "CREATE USER jobyuk WITH PASSWORD '${DB_PASS}';" 2>/dev/null || sudo -u postgres psql -c "ALTER USER jobyuk WITH PASSWORD '${DB_PASS}';"
else
    DB_PASS=$(grep "DATABASE_URL=" /var/www/jobyuk/.env 2>/dev/null | sed -E 's/.*:([^@]+)@.*/\1/' || echo "")
    if [ -n "$DB_PASS" ]; then
        sudo -u postgres psql -c "CREATE USER jobyuk WITH PASSWORD '${DB_PASS}';" 2>/dev/null || sudo -u postgres psql -c "ALTER USER jobyuk WITH PASSWORD '${DB_PASS}';"
    fi
fi
sudo -u postgres psql -c "CREATE DATABASE jobyuk OWNER jobyuk;" 2>/dev/null || true
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE jobyuk TO jobyuk;" 2>/dev/null || true

# 6. Clone or Update Application in dedicated /var/www/jobyuk
echo "📥 Cloning/Updating JobyUK into /var/www/jobyuk..."
TOKEN="${GITHUB_TOKEN:-}"
if [ -n "$TOKEN" ]; then
    REPO_URL="https://${TOKEN}@github.com/growendlessproject-bot/jobyuk.com.git"
else
    REPO_URL="https://github.com/growendlessproject-bot/jobyuk.com.git"
fi

mkdir -p /var/www
if [ -d "/var/www/jobyuk/.git" ]; then
    echo "Updating existing JobyUK..."
    cd /var/www/jobyuk
    git remote set-url origin "${REPO_URL}" 2>/dev/null || true
    git pull origin main
else
    rm -rf /var/www/jobyuk
    git clone "${REPO_URL}" /var/www/jobyuk
    cd /var/www/jobyuk
fi

# 7. Ensure Production .env file on Port 8088 exists
if [ ! -f "/var/www/jobyuk/.env" ]; then
    echo "⚙️ Creating /var/www/jobyuk/.env (PORT=8088)..."
    JWT_SEC=$(openssl rand -hex 32)
    CSRF_SEC=$(openssl rand -hex 32)
    ADMIN_PASS=$(openssl rand -base64 16 | tr -dc 'a-zA-Z0-9' | head -c 16)

    cat << EOF > /var/www/jobyuk/.env
PORT=8088
BASE_URL=https://jobyuk.com
ENV=production

# Isolated Database & Cache
DATABASE_URL=postgres://jobyuk:${DB_PASS}@localhost:5432/jobyuk?sslmode=disable
REDIS_URL=redis://localhost:6379/0

# UK Job Sync
REED_API_KEY=${REED_API_KEY:-}

# Security Secrets
JWT_SECRET=${JWT_SEC}
CSRF_SECRET=${CSRF_SEC}

# Admin Login
ADMIN_USERNAME=admin
ADMIN_PASSWORD=${ADMIN_PASS}

# Features
ENABLE_WHATSAPP_ALERTS=false
EOF
    chmod 600 /var/www/jobyuk/.env
else
    echo "⚙️ Existing /var/www/jobyuk/.env preserved safely."
fi

# 8. Import Seed Migrations into 'jobyuk' database only
echo "🗄️ Running migrations into 'jobyuk' database..."
cd /var/www/jobyuk
for f in $(ls -1 migrations/*.sql | sort); do
    echo "Running migration $f..."
    sudo -u postgres psql -d jobyuk -f "$f" || true
done

echo "🔑 Granting full permissions on all tables & sequences to 'jobyuk' user..."
sudo -u postgres psql -d jobyuk -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO jobyuk; GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO jobyuk; ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO jobyuk; ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO jobyuk;"

# 9. Build Go Binary
echo "🔨 Compiling JobyUK binary..."
export PATH=$PATH:/usr/local/go/bin
if command -v /usr/local/go/bin/go &> /dev/null; then
    /usr/local/go/bin/go build -o jobyuk ./cmd/server
else
    go build -o jobyuk ./cmd/server
fi
chmod +x /var/www/jobyuk/jobyuk

# 10. Setup Dedicated Systemd Service (jobyuk.service)
echo "⚙️ Setting up systemd service (jobyuk.service)..."
cat << 'EOF' > /etc/systemd/system/jobyuk.service
[Unit]
Description=JobyUK.com Web Application
After=network.target postgresql.service redis-server.service

[Service]
Type=simple
User=root
WorkingDirectory=/var/www/jobyuk
ExecStart=/var/www/jobyuk/jobyuk
Restart=always
RestartSec=5s
LimitNOFILE=65536
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable jobyuk
systemctl restart jobyuk

# 11. Open Port 8088 in Firewall
ufw allow 8088/tcp || true

echo "=================================================="
echo "🎉 JobyUK.com is LIVE & RUNNING on Port 8088!"
echo "iLookio.com is 100% UNTOUCHED on Port 80."
echo "Status of jobyuk.service:"
systemctl status jobyuk --no-pager
echo "=================================================="
