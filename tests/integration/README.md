# Waymon Integration Tests

This directory is reserved for integration tests.

## Status

**The integration tests need to be rewritten to work with the new clean architecture.**

The previous integration tests used the old package structure which has been refactored. The tests should be updated to use:

- `internal/adapters/out/evdev` - For input capture (evdev)
- `internal/adapters/out/input` - For input injection (Wayland)
- `internal/adapters/out/ssh` - For network transport (SSH)
- `internal/usecase/server` - For server business logic
- `internal/usecase/client` - For client business logic
- `internal/domain` - For domain types

## Running Unit Tests

Until integration tests are rewritten, use unit tests:

```bash
# Run all unit tests
make test

# Run quick tests for input adapters
make quick-test
```

## Future Test Suites

### 1. Capture Integration Test

Test evdev input capture:
- Device discovery
- Mouse/keyboard capture
- Device grabbing safety

### 2. Network Integration Test

Test SSH transport:
- Connection establishment
- Event transmission
- Reconnection logic

### 3. Wayland Injection Test

Test Wayland virtual input:
- Virtual device creation
- Mouse/keyboard injection

### 4. End-to-End Test

Test complete flow:
- Server → Network → Client
- Event routing
- Control switching
