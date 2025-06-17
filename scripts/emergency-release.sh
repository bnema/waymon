#!/bin/bash
# Waymon Emergency Release Script
# This script provides multiple ways to release control in case you get stuck

echo "🚨 Waymon Emergency Release"
echo "=========================="
echo

# Method 1: Create trigger file
echo "Method 1: Creating trigger file..."
touch /tmp/waymon-release
echo "✓ Trigger file created"

# Method 2: Send SIGUSR1 signal
echo
echo "Method 2: Sending SIGUSR1 signal..."
if pidof waymon > /dev/null; then
    sudo kill -USR1 $(pidof waymon)
    echo "✓ Signal sent to waymon process"
else
    echo "⚠ Waymon process not found"
fi

# Method 3: Send IPC command to switch to local
echo
echo "Method 3: Sending IPC command..."
echo '{"action": "switch", "data": {"action": "local"}}' | nc -N localhost 52526 2>/dev/null
echo "✓ IPC command sent"

echo
echo "🎯 Emergency release triggered!"
echo "You should regain control of your system within a few seconds."
echo
echo "If you're still stuck:"
echo "  1. Switch to another TTY (Ctrl+Alt+F2) and run: sudo pkill waymon"
echo "  2. SSH from another device and run: sudo pkill waymon"