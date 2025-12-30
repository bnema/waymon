# Waymon Integration Tests

This directory contains integration tests for Waymon, organized into tiers based on CI feasibility.

## Test Tiers

### Tier 1: CI-Friendly Tests (Always Run)

These tests run without Wayland or special hardware access:

- **IPC Tests** (`ipc_test.go`) - Unix socket IPC communication
- **SSH Tests** (`ssh_test.go`) - SSH transport and event transmission
- **Use Case Tests** (`usecase_test.go`) - Business logic helper functions

### Tier 2: Wayland Environment Tests

These tests require Wayland (can run in CI with Weston headless):

- Display detection tests (not yet implemented)
- Input injection tests (not yet implemented)

### Tier 3: Hardware-Dependent Tests

These tests require root access and physical input devices:

- Evdev capture tests (not yet implemented)

### Tier 4: End-to-End Tests

These tests require the full environment:

- Complete flow tests (not yet implemented)

## Running Tests

### Run All Integration Tests

```bash
make test-integration
```

### Run Specific Test Suites

```bash
# IPC tests only
go test -tags=integration -v ./tests/integration/... -run TestIPC

# SSH tests only
go test -tags=integration -v ./tests/integration/... -run TestSSH

# Server helper function tests
go test -tags=integration -v ./tests/integration/... -run TestServer
```

### Run with Race Detection

```bash
make test-integration-race
```

## Test Infrastructure

### helpers.go

Common test utilities:

- `testContext(t)` - Creates a context with zerolog test logger
- `tempSocketPath(t)` - Returns a unique temp socket path
- `findAvailablePort(t)` - Finds an available TCP port
- `waitForPort(t, port, timeout)` - Waits for a port to become available
- `waitForCondition(t, fn, timeout, desc)` - Waits for a condition to be true
- `generateTempSSHKeyPair(t)` - Generates ephemeral SSH keys for testing
- `generateTempHostKey(t)` - Generates temp SSH host key

### fixtures.go

Monitor layout fixtures for testing multi-monitor scenarios:

- `SingleMonitor` - Single 1920x1080 monitor
- `DualHorizontal` - Two monitors side-by-side
- `DualVertical` - Two monitors stacked
- `TripleHorizontal` - Three monitors in a row
- `QuadGrid` - 2x2 monitor grid
- `MixedScales` - Different DPI scales
- `OffsetLayout` - Non-aligned monitors
- `PortraitMode` - Rotated monitors
- `UltraWide` - 21:9 monitor
- `Monitor4K` - 4K resolution

### scenarios.go

Multi-client test scenarios:

- `TwoClientsHorizontal` - Two clients left-right
- `TwoClientsVertical` - Two clients top-bottom
- `ThreeClientsHorizontal` - Three clients in a row
- `MixedScalesAndSizes` - Different monitor configurations
- `ComplexOffice` - Complex multi-monitor setup
- `MaxClients` - Stress test with max clients

## Test Coverage Summary

| Test Suite | Test Count | Description |
|------------|------------|-------------|
| IPC | 11 tests | Unix socket protocol, client-server |
| SSH | 11 tests | Connection, events, authentication |
| Server Helpers | 6 tests | Bounds, cursors, monitors |
| **Total** | **28 tests** | |

## Adding New Tests

1. Use the `integration` build tag:
   ```go
   //go:build integration
   ```

2. Use helper functions from `helpers.go`

3. For hardware-dependent tests, use skip helpers:
   ```go
   skipIfNoWayland(t)
   skipIfNotRoot(t)
   skipIfNoInputDevices(t)
   ```

4. Run tests with `-tags=integration`:
   ```bash
   go test -tags=integration -v ./tests/integration/...
   ```
