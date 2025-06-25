# Waymon Improvement and Refactoring Plan

## Overview

This document outlines the comprehensive improvement and refactoring plan for Waymon, based on analysis of the codebase, architecture review, and identified issues. The plan is organized by priority and includes both immediate fixes and long-term architectural improvements.

## Critical Issues

### 1. Keyboard Event Handling (Cmd+1/2/3 Not Working)

**Problem**: Cmd+1/2/3 keys are not working on the client, preventing workspace switching in Hyprland.

**Root Causes**:
- Incorrect key code mapping between evdev (server) and Wayland virtual input (client)
- Hyprland intercepting keyboard shortcuts before they reach the application
- Missing keyboard shortcuts inhibitor implementation

**Solution**:
```go
// Key code mapping needs to be fixed
// evdev uses: KEY_LEFTMETA = 125, KEY_RIGHTMETA = 126
// Current mapping might be incorrect for Meta/Super/Cmd keys
```

**Implementation Steps**:
1. Add comprehensive key code translation table in `internal/input/wayland_virtual_input.go`
2. Implement keyboard shortcuts inhibitor for client-side injection
3. Add debug logging throughout the keyboard event pipeline
4. Test with various modifier combinations

### 2. SSH Server Single-Client Restriction (BROKEN)

**Problem**: Server allows multiple clients when it should restrict to one.

**Current State**:
- ClientManager exists but doesn't enforce single-client restriction
- No connection state tracking
- Missing authorized_keys validation

**Solution Architecture**:
```go
type ClientManager struct {
    mu              sync.Mutex
    activeClient    *ClientConnection
    pendingClients  []*ClientConnection
    maxClients      int // Should be 1
}
```

**Implementation Steps**:
1. Add connection limiting logic in `NewClientConnection()`
2. Implement proper state machine for client connections
3. Add authorized_keys file validation
4. Reject new connections when a client is active

## Architecture Improvements

### 3. Wish SSH Server Integration

**Current Issues**:
- Not using Wish middleware pattern effectively
- Missing authentication middleware
- No connection rate limiting

**Proposed Middleware Stack**:
```go
server, err := wish.NewServer(
    wish.WithAddress(addr),
    wish.WithHostKeyPath(hostKeyPath),
    wish.WithMiddleware(
        AuthorizedKeysMiddleware(),
        SingleClientMiddleware(),
        LoggingMiddleware(),
        BubbleteaMiddleware(),
    ),
)
```

### 4. Bubbletea UI Refactoring

**Current Issues**:
- Inconsistent use of Model/Update/View pattern
- Direct goroutine usage instead of tea.Cmd
- UI state mixed with business logic

**Improvements**:
1. Implement base model pattern consistently across all UI components
2. Use tea.Batch() for concurrent operations
3. Separate UI models from business logic
4. Add proper error handling in Update methods

### 5. Input System Architecture

**Current State**:
```
[Server]                          [Client]
evdev capture → SSH → Protocol → Wayland injection
```

**Issues**:
- No flow control or backpressure handling
- Event dropping without notification
- Missing metrics/monitoring

**Proposed Improvements**:
1. Add event prioritization (emergency > control > input)
2. Implement proper buffering with overflow handling
3. Add metrics for dropped events and latency
4. Separate capture and injection interfaces clearly

## Feature Completions

### 6. Monitor Detection & Edge Switching

**Missing Components**:
- Edge detection not integrated with input capture
- Multi-monitor arrangement not implemented
- Display backend implementations incomplete

**Implementation Plan**:
1. Complete wlr-output-management backend
2. Integrate EdgeDetector with AllDevicesCapture
3. Implement monitor arrangement detection
4. Add configuration for edge sensitivity

### 7. Protocol Buffer Enhancements

**Current Protocol Issues**:
- No versioning support
- No compression for high-frequency events
- Limited error reporting

**Proposed Changes**:
```protobuf
message InputEvent {
    uint32 version = 1;
    bool compressed = 2;
    // ... existing fields
}

message ErrorEvent {
    enum ErrorType {
        UNKNOWN = 0;
        CONNECTION_LOST = 1;
        INJECTION_FAILED = 2;
        CAPTURE_FAILED = 3;
    }
    ErrorType type = 1;
    string message = 2;
}
```

## Implementation Priority

### Phase 1: Critical Fixes (Week 1)
1. **Fix Keyboard Event Handling** ✅
   - [x] Add key code translation table
   - [x] Implement keyboard shortcuts inhibitor
   - [x] Add comprehensive debug logging
   - [x] Test with Hyprland workspace switching
   
   **Completed in commit 2fddb44f:**
   - Added human-readable key name mapping for debug logging
   - Enabled automatic keyboard shortcuts inhibitor when client is controlled
   - Added debug logging throughout keyboard event pipeline
   - Fixed Meta/Super/Cmd key logging (KEY_LEFTMETA = 125)
   - Integrated shortcuts inhibitor with control state changes

2. **Fix SSH Single-Client Restriction**
   - [ ] Implement connection limiting
   - [ ] Add connection state tracking
   - [ ] Implement authorized_keys validation
   - [ ] Add connection rejection logic

### Phase 2: Core Improvements (Week 2-3)
3. **Refactor SSH Server with Wish Patterns**
   - [ ] Implement middleware stack
   - [ ] Add authentication middleware
   - [ ] Add rate limiting
   - [ ] Improve error handling

4. **UI Architecture Refactoring**
   - [ ] Migrate to consistent base model
   - [ ] Replace goroutines with tea.Cmd
   - [ ] Separate UI and business logic
   - [ ] Add proper error handling

### Phase 3: Feature Completion (Week 4-5)
5. **Complete Input System**
   - [ ] Add flow control
   - [ ] Implement metrics
   - [ ] Add event prioritization
   - [ ] Improve error recovery

6. **Monitor Detection & Edges**
   - [ ] Complete display backends
   - [ ] Integrate edge detection
   - [ ] Add multi-monitor support
   - [ ] Implement configuration

### Phase 4: Enhancements (Week 6+)
7. **Protocol Improvements**
   - [ ] Add versioning
   - [ ] Implement compression
   - [ ] Enhance error reporting
   - [ ] Add telemetry support

## Testing Strategy

### Unit Tests
- Key code translation
- Connection limiting logic
- Event prioritization
- Protocol serialization

### Integration Tests
- End-to-end keyboard event flow
- SSH connection handling
- Multi-client rejection
- Emergency release mechanisms

### Manual Testing
- Hyprland workspace switching
- Multi-monitor setups
- Network disconnection/reconnection
- Performance under load

## Success Metrics

1. **Keyboard Functionality**: Cmd+1/2/3 works reliably for workspace switching
2. **Connection Management**: Only one client can connect at a time
3. **Stability**: No crashes or hangs during normal operation
4. **Performance**: < 1ms latency for input events
5. **User Experience**: Clear feedback and error messages

## Notes

- All changes should maintain backwards compatibility where possible
- Follow TDD approach as specified in CLAUDE.md
- Commit early and often
- No watermarks or signatures in code
- Build in dist/ directory

## References

- [Wish SSH Server](https://github.com/charmbracelet/wish)
- [Bubbletea TUI Framework](https://github.com/charmbracelet/bubbletea)
- [libwldevices-go](https://github.com/bnema/libwldevices-go)
- [Wayland Virtual Input Protocol](https://wayland.app/protocols/virtual-keyboard-unstable-v1)