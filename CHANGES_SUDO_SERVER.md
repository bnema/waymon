# Server Sudo Requirement Changes

## Summary of Changes

### 1. Server Command (`cmd/server.go`)
- Added check for root privileges (`os.Geteuid() == 0`) at the start of `runServer`
- Returns error "waymon server must be run with sudo" if not running as root
- Sets config path to `/etc/waymon/waymon.toml` before initializing config
- Added import for `path/filepath`
- Updated `ensureServerConfig` to create `/etc/waymon` directory when needed

### 2. Configuration (`internal/config/config.go`)
- Added `SetConfigPath` function to override the config path
- Added `configPathOverride` variable to store the override
- Modified `Init()` to use the override path if set (no fallback for server mode)
- Updated `GetConfigPath()` to return override path if set
- Changed default SSH paths in `DefaultConfig` to use `/etc/waymon/` instead of `~/.config/waymon/`

### 3. IPC Socket (`internal/ipc/socket.go`)
- Modified `getSocketPath` to return `/tmp/waymon.sock` when running as root (server mode)
- User mode continues to use `/tmp/waymon-{username}.sock`
- Updated socket permissions: 0666 for server mode (allows all users), 0600 for user mode

### 4. IPC Client (`internal/ipc/client.go`)
- Updated `NewClient` to try the server socket (`/tmp/waymon.sock`) first
- Falls back to user-specific socket if server socket doesn't exist
- This allows clients to automatically connect to the system-wide server

## Benefits

1. **Security**: Server runs with proper permissions to access input devices
2. **Consistency**: System-wide configuration managed by administrators
3. **Predictability**: Socket always at `/tmp/waymon.sock` for easy client connection
4. **Multi-user**: All users on the system can send switch commands to the server

## Usage

```bash
# Start server (requires sudo)
sudo waymon server

# Client can connect without sudo
waymon client --host server.local:52525

# Switch commands work for any user
waymon switch
```