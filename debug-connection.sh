#!/bin/bash

# Debug script to test server/client connection with proper logging

echo "Waymon Connection Debug Script"
echo "=============================="
echo ""

# Kill any existing waymon processes
echo "1. Killing any existing waymon processes..."
pkill -f "waymon server" 2>/dev/null
pkill -f "waymon client" 2>/dev/null
sleep 1

# Clear the log file
echo "2. Clearing log file..."
rm -f ~/.local/share/waymon/waymon.log

# Start server in background
echo "3. Starting server in background (no-tui mode for debugging)..."
LOG_LEVEL=debug ./dist/waymon server --no-tui &
SERVER_PID=$!
echo "   Server PID: $SERVER_PID"

# Wait for server to start
echo "4. Waiting for server to initialize..."
sleep 3

# Show server logs
echo "5. Server logs so far:"
echo "   -------------------"
if [ -f ~/.local/share/waymon/waymon.log ]; then
    grep "SERVER:" ~/.local/share/waymon/waymon.log | tail -20
fi
echo ""

# Test client connection
echo "6. Testing client connection..."
LOG_LEVEL=debug timeout 5 ./dist/waymon client -H localhost:52525 &
CLIENT_PID=$!

# Wait for connection
sleep 2

# Show updated logs
echo "7. Logs after client connection:"
echo "   -----------------------------"
if [ -f ~/.local/share/waymon/waymon.log ]; then
    tail -30 ~/.local/share/waymon/waymon.log | grep -E "(SERVER:|CLIENT:)"
fi

# Cleanup
echo ""
echo "8. Cleaning up..."
kill $SERVER_PID 2>/dev/null
kill $CLIENT_PID 2>/dev/null

echo ""
echo "Done! Check the output above for connection details."
echo "Look for:"
echo "  - 'SSH session started' messages"
echo "  - 'OnClientConnected callback' messages"
echo "  - 'Client connected from' messages"