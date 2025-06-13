#!/bin/bash

echo "Testing Waymon Server TUI..."
echo "================================"
echo ""
echo "This script will test different TUI modes for the Waymon server."
echo "Press Ctrl+C or 'q' in the TUI to exit each test."
echo ""

# Function to run a test
run_test() {
    local name="$1"
    local cmd="$2"
    
    echo ""
    echo "Test: $name"
    echo "Command: $cmd"
    echo "Press Enter to start..."
    read -r
    
    # Clear the log file
    rm -f ~/.local/share/waymon/waymon.log
    
    # Run the command
    eval "$cmd"
    
    # Show log results
    echo ""
    echo "Log output:"
    echo "----------"
    if [ -f ~/.local/share/waymon/waymon.log ]; then
        tail -20 ~/.local/share/waymon/waymon.log
    else
        echo "No log file found"
    fi
}

# Test 1: No TUI mode (baseline)
run_test "No TUI mode" "LOG_LEVEL=debug ./dist/waymon server --no-tui"

# Test 2: Debug TUI mode
run_test "Debug TUI mode" "LOG_LEVEL=debug ./dist/waymon server --debug-tui"

# Test 3: Full-screen TUI mode
run_test "Full-screen TUI mode" "LOG_LEVEL=debug ./dist/waymon server"

echo ""
echo "All tests completed!"