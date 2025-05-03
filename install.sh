#!/bin/bash

# Exit on any error
set -e

# Default installation directory
INSTALL_DIR="/opt/geoip-api"
# Default host (empty means all interfaces)
HOST=""
# Default port
PORT="5324"
# Default user and group
USER="geoip"
GROUP="geoip"
# Default binary name (will try to auto-detect if not specified)
BINARY_NAME=""

# Process command line arguments
while [ $# -gt 0 ]; do
  case "$1" in
    --install-dir=*)
      INSTALL_DIR="${1#*=}"
      ;;
    --host=*)
      HOST="${1#*=}"
      ;;
    --port=*)
      PORT="${1#*=}"
      ;;
    --user=*)
      USER="${1#*=}"
      ;;
    --group=*)
      GROUP="${1#*=}"
      ;;
    --binary=*)
      BINARY_NAME="${1#*=}"
      ;;
    --help)
      echo "Usage: $0 [OPTIONS]"
      echo "Install the geoip-api service."
      echo ""
      echo "Options:"
      echo "  --install-dir=DIR    Installation directory (default: /opt/geoip-api)"
      echo "  --host=HOST          Host to bind to (default: empty, binds to all interfaces)"
      echo "  --port=PORT          Port to listen on (default: 5324)"
      echo "  --user=USER          User to run the service as (default: geoip)"
      echo "  --group=GROUP        Group to run the service as (default: geoip)"
      echo "  --binary=NAME        Specific binary name to use (e.g., geoip-api_linux_arm64)"
      echo "  --help               Display this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      echo "Use --help for usage information."
      exit 1
      ;;
  esac
  shift
done

# Check if running as root
if [ "$(id -u)" -ne 0 ]; then
  echo "This script must be run as root. Please use sudo or run as root user."
  exit 1
fi

echo "Installing geoip-api..."
echo "Installation directory: $INSTALL_DIR"
echo "Host: $HOST"
echo "Port: $PORT"
echo "User: $USER"
echo "Group: $GROUP"

# Create installation directory
mkdir -p "$INSTALL_DIR/maxmind_db"

# Create config.json
cat > "$INSTALL_DIR/config.json" << EOF
{
  "host": "$HOST",
  "port": "$PORT"
}
EOF

# Find the appropriate binary
SOURCE_BINARY=""
if [ -n "$BINARY_NAME" ] && [ -f "$BINARY_NAME" ]; then
  # Use the specified binary
  SOURCE_BINARY="$BINARY_NAME"
elif [ -f "geoip-api" ]; then
  # Use the default binary name if available
  SOURCE_BINARY="geoip-api"
else
  # Try to find any matching binary with a platform-specific name
  FOUND_BINARY=$(find . -maxdepth 1 -name "geoip-api*" -type f -executable | head -n 1)
  if [ -n "$FOUND_BINARY" ]; then
    SOURCE_BINARY="$FOUND_BINARY"
  fi
fi

# Copy files
if [ -n "$SOURCE_BINARY" ]; then
  echo "Using binary: $SOURCE_BINARY"
  cp "$SOURCE_BINARY" "$INSTALL_DIR/geoip-api"
  chmod +x "$INSTALL_DIR/geoip-api"
else
  echo "Error: No geoip-api executable found in the current directory."
  echo "Please specify the binary using --binary=FILENAME or ensure a binary with name"
  echo "geoip-api or matching pattern geoip-api* exists in the current directory."
  exit 1
fi

# Create systemd service file
cat > /etc/systemd/system/geoip-api.service << EOF
[Unit]
Description=GeoIP API Service
After=network.target

[Service]
Type=simple
User=$USER
Group=$GROUP
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/geoip-api -config $INSTALL_DIR/config.json
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=geoip-api

# Hardening
ProtectSystem=full
PrivateTmp=true
NoNewPrivileges=true
ProtectHome=true
ProtectControlGroups=true
ProtectKernelModules=true
ProtectKernelTunables=true
RestrictAddressFamilies=AF_INET AF_INET6
RestrictNamespaces=true
RestrictRealtime=true
SystemCallArchitectures=native

[Install]
WantedBy=multi-user.target
EOF

# Create system user for the service if it doesn't exist
if ! id -u $USER > /dev/null 2>&1; then
  useradd -r -s /bin/false -d "$INSTALL_DIR" $USER
fi

# Set proper ownership
chown -R $USER:$GROUP "$INSTALL_DIR"

echo "Setting up systemd service..."
systemctl daemon-reload
# Don't automatically enable or start the service
# Let the user decide when to do this

echo "Installation completed successfully!"
echo "To enable and start the service, run:"
echo "  systemctl enable geoip-api"
echo "  systemctl start geoip-api"
echo ""
echo "The MaxMind databases will be downloaded on first run."
echo "If you have custom database files, place them in $INSTALL_DIR/maxmind_db/"
echo "The service will run on http://$HOST:$PORT/ (empty host means all interfaces)"