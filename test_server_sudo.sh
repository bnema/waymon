#!/bin/bash
# Test script to verify server sudo requirement and system config

echo "=== Testing Waymon Server Sudo Requirements ==="
echo

# Test 1: Running server without sudo should fail
echo "Test 1: Running server without sudo (should fail)"
./waymon server --no-tui 2>&1 | head -5
echo

# Test 2: Check config path when running with sudo
echo "Test 2: Config path for server mode"
echo "Expected: /etc/waymon/waymon.toml"
echo

# Test 3: Socket path
echo "Test 3: Socket path for server (running as root)"
echo "Expected: /tmp/waymon.sock"
echo

echo "Note: To fully test, run: sudo ./waymon server --no-tui"
echo "This will:"
echo "1. Require sudo to run"
echo "2. Use /etc/waymon/waymon.toml for config"
echo "3. Create socket at /tmp/waymon.sock"