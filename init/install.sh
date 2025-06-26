#!/bin/bash
set -e

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "Please run as root (use sudo)"
    exit 1
fi

echo "Installing Waymon systemd service..."

# Create directories
echo "Creating directories..."
mkdir -p /etc/waymon
mkdir -p /usr/local/bin

# Copy binary
if [ -f "../waymon" ]; then
    echo "Copying waymon binary..."
    cp ../waymon /usr/local/bin/
    chmod +x /usr/local/bin/waymon
elif [ -f "../dist/waymon" ]; then
    echo "Copying waymon binary from dist..."
    cp ../dist/waymon /usr/local/bin/
    chmod +x /usr/local/bin/waymon
else
    echo "Error: waymon binary not found. Please build it first."
    exit 1
fi

# Generate SSH host key if not exists
if [ ! -f /etc/waymon/ssh_host_key ]; then
    echo "Generating SSH host key..."
    ssh-keygen -t ed25519 -f /etc/waymon/ssh_host_key -N ""
fi

# Create authorized_keys file if not exists
if [ ! -f /etc/waymon/authorized_keys ]; then
    echo "Creating authorized_keys file..."
    touch /etc/waymon/authorized_keys
    chmod 600 /etc/waymon/authorized_keys
    echo "Note: Add client SSH public keys to /etc/waymon/authorized_keys"
fi

# Copy systemd service
echo "Installing systemd service..."
cp waymon.service /etc/systemd/system/
systemctl daemon-reload

echo ""
echo "Installation complete!"
echo ""
echo "To start the service:"
echo "  sudo systemctl enable waymon"
echo "  sudo systemctl start waymon"
echo ""
echo "To check status:"
echo "  sudo systemctl status waymon"
echo "  waymon status"
echo ""
echo "Remember to add client SSH keys to /etc/waymon/authorized_keys"