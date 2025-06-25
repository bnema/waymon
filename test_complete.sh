#!/bin/bash

echo "=== Waymon Complete Test ==="
echo "This will test client connection UI updates and log forwarding"
echo

# Set debug log level
export LOG_LEVEL=DEBUG

echo "1. Starting server with UI (not daemon mode)..."
echo "   Watch for client appearing in the UI when connected"
echo

echo "Please start the server manually in another terminal with:"
echo "sudo LOG_LEVEL=DEBUG ./dist/waymon server"
echo
echo "Press Enter when server is running..."
read

echo "2. Starting client..."
./dist/waymon client --host localhost:52525 2>&1 | tee client_output.log &
CLIENT_PID=$!

echo "Client started with PID: $CLIENT_PID"
echo

echo "3. Waiting 5 seconds for connection..."
sleep 5

echo "4. Checking for client logs on server..."
if sudo ls /var/log/waymon/waymon_client_*.log 2>/dev/null; then
    echo "SUCCESS: Client log files found!"
    echo "Latest client log contents:"
    sudo tail -20 /var/log/waymon/waymon_client_*.log
else
    echo "ISSUE: No client log files found"
    echo "Checking server debug logs..."
    sudo grep -E "(handleClientLog|Received log event|OnClientConnected)" /var/log/waymon/waymon.log | tail -20
fi

echo
echo "5. Checking client output for log forwarding..."
if grep -q "LOG-FORWARDER" client_output.log; then
    echo "SUCCESS: Log forwarding is active"
    grep "LOG-FORWARDER" client_output.log | head -5
else
    echo "ISSUE: No log forwarding messages found"
fi

echo
echo "6. Killing client..."
kill $CLIENT_PID 2>/dev/null

echo
echo "Test complete. Please check:"
echo "- Did the client appear in the server UI?"
echo "- Can you switch to the client with key 1?"
echo "- Are client logs being saved on the server?"

rm -f client_output.log