#!/bin/bash
set -e

echo "🔑 Ensuring database permissions for jobyuk user..."
sudo -u postgres psql -d jobyuk << 'EOF'
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO jobyuk;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO jobyuk;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO jobyuk;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO jobyuk;
EOF

echo "🔄 Restarting JobyUK service..."
systemctl restart jobyuk

echo "============================================="
echo "🎉 SUCCESS: JobyUK service is active and running!"
echo "============================================="
