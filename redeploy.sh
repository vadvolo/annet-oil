#!/bin/bash
# Redeploy script for Annet Oil

set -e

echo "[INFO] Starting redeploy process..."

# Pull latest changes
echo "[INFO] Pulling latest changes..."
cd /root/annet-oil
git pull

# Build Go binary
echo "[INFO] Building annet-oil binary..."
make build

# Install/update files
echo "[INFO] Installing files..."
cd configs/systemd
./install-annet-oil.sh

# Rebuild MCP server
echo "[INFO] Rebuilding MCP server..."
cd /opt/annet-oil/mcp-annet-oil
npm install
npm run build

# Restart services
echo "[INFO] Restarting services..."
systemctl daemon-reload
systemctl restart annet-oil
systemctl restart mcp-annet-oil

# Check status
echo "[INFO] Service status:"
systemctl status annet-oil --no-pager | head -15
echo ""
systemctl status mcp-annet-oil --no-pager | head -15

echo "[INFO] Redeploy complete!"
echo ""
echo "Check logs with:"
echo "  journalctl -u annet-oil -f"
echo "  journalctl -u mcp-annet-oil -f"