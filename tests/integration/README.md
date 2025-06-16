# Waymon Integration Tests

This directory contains integration tests for the Waymon application. These tests verify the complete functionality of various components working together.

## Test Suites

### 1. Capture Integration Test (`capture/`)

Verifies that the input capture backend correctly captures mouse and keyboard events using evdev.

**What it tests:**
- Device discovery and initialization
- Mouse movement, button, and scroll capture
- Keyboard key press/release capture
- Device targeting (local vs remote control)
- Performance and event throughput

### 2. Network Integration Test (`network/`)

Tests the SSH transport layer and client-server communication.

**What it tests:**
- SSH connection establishment
- Bidirectional event flow
- Protocol buffer serialization/deserialization
- High throughput event streaming
- Connection reconnection logic
- End-to-end flow with managers

### 3. Wayland Integration Test (`wayland/`)

Tests the Wayland virtual input injection on the client side.

**What it tests:**
- Virtual device creation (pointer & keyboard)
- Mouse movement and click injection
- Keyboard event injection
- Combined mouse+keyboard events (e.g., Ctrl+Click)

## Running the Tests

### Individual Test Suites

```bash
# Capture tests (requires input device access)
make test-capture                    # Basic non-interactive
make test-capture-interactive        # Interactive with user input

# Network tests
make test-network                    # Tests SSH transport

# Wayland tests (requires Wayland session)
make test-wayland                    # Tests virtual input injection
```

### Run All Integration Tests

```bash
# Run all integration tests
make test-integration
```

### Advanced Usage

```bash
# Run capture test with custom duration
go run tests/integration/capture/main.go -v -i -d 30s

# Run network test on custom port
go run tests/integration/network/main.go -v -port 52527

# Run Wayland test with verbose output
go run tests/integration/wayland/main.go -v
```

### Safety Features

The test has built-in safety to prevent losing control of your input devices:

1. **5-Second Auto-Release**: Devices are automatically released after 5 seconds
2. **Emergency Release**: Press ESC to immediately release all devices
3. **Brief Grab Periods**: Device targeting test only grabs for 200ms

### Test Flags

- `-v`: Verbose output - shows all captured events
- `-i`: Run interactive tests that require user input
- `-d <duration>`: Set duration for each interactive test (default: 10s)

### What the Tests Verify

1. **Basic Capture**: Verifies the backend initializes and can capture events
2. **Device Targeting**: Tests switching between local and remote control modes
3. **Event Types**: Ensures different event types are properly detected
4. **Performance**: Measures event processing throughput
5. **Interactive Mouse** (with -i): Tests real mouse movement, clicks, and scrolling
6. **Interactive Keyboard** (with -i): Tests real keyboard input
7. **Interactive Combined** (with -i): Tests simultaneous mouse and keyboard input

### Requirements

- **Input device access**: Read/write access to `/dev/input/event*` devices
  - Run with sudo, OR
  - Add your user to the 'input' group: `sudo usermod -a -G input $USER`
  - Set up appropriate udev rules
- **Linux**: Uses evdev for input capture
- **Physical input devices**: Mouse and keyboard for interactive tests

### Expected Results

When running interactive tests, you should see:
- Mouse movement events captured at ~60 FPS
- Mouse button press/release events for all buttons
- Keyboard press/release events with proper keycodes
- Correct event ordering and timestamps
- Proper target switching between local and remote modes

### Troubleshooting

If tests fail:
1. Check input device permissions:
   - Try: `ls -la /dev/input/event*`
   - Ensure your user has read/write access
2. If permission denied:
   - Run with sudo: `sudo make test-capture`
   - Or add user to input group: `sudo usermod -a -G input $USER` (then logout/login)
3. Verify no other applications have exclusive access to input devices
4. For interactive tests, ensure you're actively using mouse/keyboard during the test period