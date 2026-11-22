#!/usr/bin/env bash
set -e

echo "Setting up JobyUK.com Auto-Updater..."

cat << 'EOF' > /usr/local/bin/jobyuk-autoupdate.sh
#!/usr/bin/env bash
set -e

cd /var/www/jobyuk 2>/dev/null || exit 0
export PATH=$PATH:/usr/local/go/bin

git fetch origin main --quiet 2>/dev/null || exit 0

LOCAL=$(git rev-parse HEAD 2>/dev/null || echo "")
REMOTE=$(git rev-parse origin/main 2>/dev/null || echo "")

if [ -n "$LOCAL" ] && [ -n "$REMOTE" ] && [ "$LOCAL" != "$REMOTE" ]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] 🚀 New commit detected: $REMOTE"
    git pull origin main --quiet
    
    # Safe compile to temp binary
    if go build -o /var/www/jobyuk/jobyuk.new ./cmd/server; then
        mv /var/www/jobyuk/jobyuk.new /var/www/jobyuk/jobyuk
        chmod +x /var/www/jobyuk/jobyuk
        systemctl restart jobyuk
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] ✅ Updated and restarted JobyUK successfully!"
    else
        echo "[$(date '+%Y-%m-%d %H:%M:%S')] ❌ Build failed! Keeping existing version running."
        rm -f /var/www/jobyuk/jobyuk.new
    fi
fi
EOF

chmod +x /usr/local/bin/jobyuk-autoupdate.sh

cat << 'EOF' > /etc/cron.d/jobyuk-autoupdate
*/3 * * * * root /usr/local/bin/jobyuk-autoupdate.sh >> /var/log/jobyuk-autoupdate.log 2>&1
EOF

chmod 644 /etc/cron.d/jobyuk-autoupdate

echo "=========================================================="
echo "🎉 JobyUK Auto-Updater is ACTIVE! (Checks every 3 minutes)"
echo "From now on, just 'git push' from local, VPS updates itself!"
echo "=========================================================="
