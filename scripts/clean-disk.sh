#!/bin/bash
# ==========================================
# Automated Server Disk Cleanup Script
# Cleans disk space, logs, cache, & runner artifacts
# ==========================================

echo "=========================================="
echo "Starting Deep Disk Cleanup: $(date)"
echo "Disk space BEFORE cleanup:"
df -h /
echo "Top directory breakdown:"
du -sh /root /var /usr /tmp /var/log /var/lib 2>/dev/null || true

# 1. Truncate active uncompressed log files (Nginx, Syslog, JobyUK)
echo "1. Truncating active log files..."
find /var/log -type f -name "*.log" -size +5M -exec truncate -s 0 {} + 2>/dev/null || true
truncate -s 0 /var/log/syslog /var/log/messages /var/log/nginx/*.log /var/log/jobyuk*.log /var/log/jobyuk*.log 2>/dev/null || true
if command -v systemctl &> /dev/null; then
    systemctl reload nginx 2>/dev/null || true
fi

# 2. Remove compressed and rotated log files & crash reports
echo "2. Removing rotated log files & crash reports..."
rm -rf /var/log/*.gz /var/log/*.[0-9] /var/log/*/*.gz /var/crash/* 2>/dev/null || true

# 3. Systemd journal logs (keep max 10MB, max 1 hour)
echo "3. Vacuuming journal logs..."
journalctl --vacuum-time=1h > /dev/null 2>&1 || true
journalctl --vacuum-size=10M > /dev/null 2>&1 || true

# 4. Clean runner diagnostics & other runner workspaces (preserve active runner _temp for post-job hooks)
echo "4. Cleaning runner diagnostics and other runner work..."
rm -rf /root/actions-runner*/_diag/* 2>/dev/null || true
rm -rf /root/actions-runner-iremotly/_work/* 2>/dev/null || true

# 5. Clean Go zip download archives (keeps source modules, removes heavy .zip duplicates)
echo "5. Cleaning Go zip downloads..."
chmod -R u+w /root/go/pkg/mod/cache 2>/dev/null || true
rm -rf /root/go/pkg/mod/cache/download 2>/dev/null || true

# 6. Clean APT cache (with timeout to prevent hangs)
echo "6. Cleaning APT cache..."
timeout 30 apt-get autoremove --purge -y > /dev/null 2>&1 || true
apt-get clean > /dev/null 2>&1 || true
rm -rf /var/cache/apt/archives/* /var/lib/apt/lists/* 2>/dev/null || true

# 7. Clear temporary directories
echo "7. Clearing temporary files..."
rm -rf /tmp/* /var/tmp/* 2>/dev/null || true

# 8. Docker cleanup (if docker is installed)
if command -v docker &> /dev/null; then
    echo "8. Pruning docker unused objects..."
    docker system prune -af --volumes > /dev/null 2>&1 || true
fi

echo "=========================================="
echo "Disk space AFTER cleanup:"
df -h /
echo "Cleanup finished successfully."
echo "=========================================="
