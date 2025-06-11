#!/bin/bash
# Setup script for uinput permissions

set -e

echo "Waymon uinput Setup Script"
echo "========================="
echo

# Check if running as root
if [ "$EUID" -eq 0 ]; then 
   echo "Please run this script as a normal user (not root)"
   echo "The script will use sudo when needed"
   exit 1
fi

# Check if uinput module is loaded
if ! lsmod | grep -q uinput; then
    echo "Loading uinput module..."
    sudo modprobe uinput
    echo "✓ uinput module loaded"
else
    echo "✓ uinput module already loaded"
fi

# Check if /dev/uinput exists
if [ ! -c /dev/uinput ]; then
    echo "✗ /dev/uinput not found - this might be a problem"
    exit 1
else
    echo "✓ /dev/uinput exists"
fi

# Check current permissions
echo
echo "Current /dev/uinput permissions:"
ls -la /dev/uinput

# Offer setup options
echo
echo "Setup Options:"
echo "1. Quick test (temporary - chmod 666)"
echo "2. Add user to input group (recommended)"
echo "3. Create udev rule (permanent, best option)"
echo "4. Check current setup only"
echo
read -p "Choose option (1-4): " choice

case $choice in
    1)
        echo "Setting temporary permissions..."
        sudo chmod 666 /dev/uinput
        echo "✓ Temporary permissions set"
        echo "Note: This will reset on reboot"
        ;;
    2)
        echo "Adding $USER to input group..."
        sudo usermod -a -G input $USER
        echo "✓ User added to input group"
        echo "Note: You need to log out and back in for this to take effect"
        ;;
    3)
        echo "Creating udev rule..."
        echo 'KERNEL=="uinput", GROUP="input", MODE="0660"' | sudo tee /etc/udev/rules.d/99-uinput.rules
        sudo udevadm control --reload-rules
        sudo udevadm trigger
        echo "✓ udev rule created"
        echo "Note: You may still need to add your user to the input group (option 2)"
        ;;
    4)
        echo "Just checking current setup"
        ;;
    *)
        echo "Invalid option"
        exit 1
        ;;
esac

# Test access
echo
echo "Testing access..."
if timeout 1 bash -c "echo -n '' > /dev/uinput 2>/dev/null"; then
    echo "✓ You have write access to /dev/uinput"
    echo
    echo "You can now run: go run cmd/test-input/main.go"
else
    echo "✗ No write access to /dev/uinput"
    echo
    echo "If you chose option 2, you need to log out and back in"
    echo "Otherwise, try option 1 for quick testing"
fi