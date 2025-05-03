#!/bin/bash

# Exit on any error
set -e

# Default installation directory
INSTALL_DIR="/opt/geoip-api"
# Default user
USER="geoip"
# Whether to remove the user account
REMOVE_USER=false

# Process command line arguments
while [ $# -gt 0 ]; do
  case "$1" in
    --install-dir=*)
      INSTALL_DIR="${1#*=}"
      ;;
    --user=*)
      USER="${1#*=}"
      ;;
    --remove-user)
      REMOVE_USER=true
      ;;
    --help)
      echo "Usage: $0 [OPTIONS]"
      echo "Uninstall the geoip-api service."
      echo ""
      echo "Options:"
      echo "  --install-dir=DIR    Installation directory (default: /opt/geoip-api)"
      echo "  --user=USER          User the service runs as (default: geoip)"
      echo "  --remove-user        Also remove the service user account"
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

echo "Uninstalling geoip-api..."
echo "Installation directory: $INSTALL_DIR"
echo "User: $USER"
echo "Remove user: $REMOVE_USER"

# Stop and disable the service if it exists
if systemctl list-unit-files | grep -q geoip-api.service; then
  echo "Stopping and disabling the geoip-api service..."
  systemctl stop geoip-api.service || true
  systemctl disable geoip-api.service || true
  echo "Removing systemd service file..."
  rm -f /etc/systemd/system/geoip-api.service
  systemctl daemon-reload
else
  echo "Service not found, skipping service removal."
fi

# Remove the installation directory
if [ -d "$INSTALL_DIR" ]; then
  echo "Removing installation directory..."
  rm -rf "$INSTALL_DIR"
else
  echo "Installation directory not found, skipping."
fi

# Remove the user if requested
if [ "$REMOVE_USER" = true ] && id -u "$USER" > /dev/null 2>&1; then
  echo "Removing user $USER..."
  userdel "$USER"
fi

echo "Uninstallation completed successfully!"