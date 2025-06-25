#!/bin/bash

# Test client logging functionality

echo "Testing client log forwarding..."

# Set debug log level
export LOG_LEVEL=DEBUG

# Start the client with a test connection
echo "Starting client in debug mode..."
./dist/waymon client --host localhost:52525 &
CLIENT_PID=$!

echo "Client started with PID: $CLIENT_PID"
echo "Waiting 5 seconds for connection and log forwarding..."
sleep 5

# Check if client logs were created on the server
if [ -f "/var/log/waymon/waymon_client_*.log" ]; then
    echo "SUCCESS: Client log file found!"
    echo "Contents of client log files:"
    ls -la /var/log/waymon/waymon_client_*.log
    echo "---"
    tail -n 20 /var/log/waymon/waymon_client_*.log
else
    echo "ERROR: No client log files found in /var/log/waymon/"
    echo "Checking server logs for client log handling..."
    grep -i "client log" /var/log/waymon/waymon.log | tail -20
fi

# Kill the client
kill $CLIENT_PID 2>/dev/null

echo "Test complete."