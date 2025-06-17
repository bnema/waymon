# Waymon

**Wayland Mouse Over Network** - A seamless input sharing solution for Wayland that allows you to control multiple computers with a single mouse and keyboard, similar to Synergy/Barrier but built specifically for modern Wayland compositors.

> ⚠️ **Early Development Stage**: This project is in active development. While the core functionality is working, expect breaking changes and rough edges. Contributions and feedback are welcome!

## Features

### Working
- ✅ **Mouse button events** (left, right, middle click)
- ✅ **Keyboard input** forwarding
- ✅ **Secure SSH transport** with key-based authentication
- ✅ **Real-time TUI** for monitoring connections and status
- ✅ **Automatic input release** on client disconnect
- ✅ **Emergency release** mechanisms (Ctrl+ESC, timeout, manual)

### Todo
- 🚧 Screen edge detection and switching
- 🚧 Absolute mouse positioning
- 🚧 Cursor constraints
- 🚧 Improved display boundary detection
- 🚧 Edges detection and switching
- 🚧 Multiple monitor support on client
- 🚧 Multiple simultaneous client support
- 🚧 Clipboard synchronization (TBD)

## How It Works

Waymon captures input events on the server (the one with the physical keyboard and mouse) using Linux's evdev interface, forwards them over an encrypted SSH connection to the client, and injects them using Wayland's virtual input protocols.

## Requirements

### Server (Computer being controlled)
- Linux with Wayland compositor
- Root/sudo access (for evdev input capture)
- SSH server
- Port 52525 available (configurable)

### Client (Computer you're controlling from)
- Linux with Wayland compositor supporting:
  - `zwp_virtual_pointer_v1` protocol
  - `zwp_virtual_keyboard_v1` protocol
- SSH client with key-based authentication

### Tested Compositors
- ✅ Hyprland
- ⚠️ GNOME Wayland (needs testing)
- ⚠️ KDE Wayland (needs testing)

## Installation

### From Source (Recommended during alpha)

```bash
git clone https://github.com/bnema/waymon.git
cd waymon
go build -o waymon .
sudo mv waymon /usr/local/bin/
```

### The Go Install way
```bash
go install github.com/bnema/waymon@latest
```

## Quick Start

1. **On the server** (computer to be controlled):
   ```bash
   sudo waymon server
   ```

2. **On the client** (computer you're controlling from):
   ```bash
   waymon client --host SERVER_IP:52525
   ```

3. **First connection**: You'll be prompted on the server to approve the client's SSH key
## Server Controls

When running the server, you can use these keyboard shortcuts in the TUI:

- **0** or **ESC**: Return control to local (server) system
- **1-5**: Switch control to connected client by number
- **R**: Manual emergency release (when controlling a client)
- **Tab**: Cycle through connected clients
- **G/g**: Navigate logs (bottom/top)
- **Q**: Quit server

## Emergency Release

If input gets stuck while controlling a client, Waymon provides multiple release mechanisms:

1. **Ctrl+ESC**: Emergency key combination (when grabbed)
2. **R key**: Manual release in server TUI
3. **30-second timeout**: Automatic release after inactivity
4. **Client disconnect**: Automatic release when client disconnects
5. **SIGUSR1**: Send signal to server process: `sudo pkill -USR1 waymon`
6. **Touch file**: Create `/tmp/waymon-release` to trigger release

## Configuration

Waymon uses TOML configuration files in the following order of precedence:
1. `/etc/waymon/waymon.toml` (system-wide, preferred for servers)
2. `~/.config/waymon/waymon.toml` (user-specific)
3. `./waymon.toml` (current directory)

### Server Configuration

Server mode automatically creates `/etc/waymon/waymon.toml` on first startup with default values. You can also create it manually:

```toml
[server]
# Port to listen on for SSH connections
port = 52525

# Bind address - use "0.0.0.0" to listen on all interfaces
bind_address = "0.0.0.0"

# Human-readable server name (defaults to hostname)
name = "my-desktop"

# Maximum number of simultaneous client connections
max_clients = 1

# Path to SSH host key file (created automatically if doesn't exist)
ssh_host_key_path = "/etc/waymon/host_key"

# Path to SSH authorized_keys file for client authentication
ssh_authorized_keys_path = "/etc/waymon/authorized_keys"

# List of allowed SSH key fingerprints (empty = allow all authorized keys)
ssh_whitelist = []

# Only allow SSH keys in the whitelist (requires ssh_whitelist to be set)
ssh_whitelist_only = true

[logging]
# Enable file logging to /var/log/waymon/waymon.log (when run with sudo)
file_logging = true

# Log level: "DEBUG", "INFO", "WARN", "ERROR" (empty = use LOG_LEVEL env var)
log_level = ""

# Known hosts for quick client connections (managed via CLI)
[[hosts]]
name = "laptop"
address = "192.168.1.101:52525"
position = "right"  # left, right, top, bottom
```

### Client Configuration

Client mode uses in-memory defaults. To customize settings, create `~/.config/waymon/waymon.toml` manually or run `waymon config init`:

```toml
[client]
# Default server address to connect to
server_address = ""

# Automatically connect to server on startup
auto_connect = false

# Delay in seconds before attempting to reconnect after disconnect
reconnect_delay = 5

# Pixel threshold for screen edge detection
edge_threshold = 5

# Legacy screen position setting (deprecated - use edge_mappings instead)
screen_position = "right"

# Hotkey modifier combination for manual switching
hotkey_modifier = "ctrl+alt"

# Hotkey key for manual switching (combined with modifier)
hotkey_key = "s"

# Path to SSH private key for server authentication (empty = use SSH agent)
ssh_private_key = ""

# Monitor-specific edge mappings for multi-monitor setups
[[client.edge_mappings]]
monitor_id = "primary"  # Monitor ID, "primary", or "*" for any monitor
edge = "right"          # "left", "right", "top", "bottom"
host = "server-name"    # Host name or IP:port to connect to
description = "Main server on the right"

[logging]
# Enable file logging to ~/.local/share/waymon/waymon.log
file_logging = true

# Log level: "DEBUG", "INFO", "WARN", "ERROR" (empty = use LOG_LEVEL env var)
log_level = ""

# Known hosts for quick connections
[[hosts]]
name = "desktop"
address = "192.168.1.100:52525"
position = "left"
```

### Complete Configuration Reference

Here's a complete configuration file with all available options and their defaults:

```toml
[server]
port = 52525                                      # SSH server port
bind_address = "0.0.0.0"                         # Bind to all interfaces
name = "hostname"                                 # Server name (auto-detected)
max_clients = 1                                   # Maximum concurrent clients
ssh_host_key_path = "/etc/waymon/host_key"        # SSH host key location
ssh_authorized_keys_path = "/etc/waymon/authorized_keys"  # SSH authorized keys
ssh_whitelist = []                                # Allowed key fingerprints
ssh_whitelist_only = true                         # Only allow whitelisted keys

[client]
server_address = ""                               # Default server to connect to
auto_connect = false                              # Auto-connect on startup
reconnect_delay = 5                               # Reconnection delay (seconds)
edge_threshold = 5                                # Edge detection sensitivity (pixels)
screen_position = "right"                         # Legacy position (deprecated)
hotkey_modifier = "ctrl+alt"                      # Hotkey modifier keys
hotkey_key = "s"                                  # Hotkey activation key
ssh_private_key = ""                              # SSH private key path
edge_mappings = []                                # Monitor-specific edge configs

[logging]
file_logging = true                               # Enable file logging
log_level = ""                                    # Log level (empty = env var)

hosts = []                                        # Known hosts list (name, address, position)
```

## Troubleshooting

### Debug Logging

Enable debug logs by either:

1. Setting the environment variable:
```bash
sudo LOG_LEVEL=DEBUG waymon server
```

2. Or in the config file:
```toml
[logging]
log_level = "DEBUG"
```

Log files are stored in:
- Server: `/var/log/waymon/waymon.log` (when run with sudo)
- Client: `~/.local/share/waymon/waymon.log`

To disable file logging (logs only in TUI):
```toml
[logging]
file_logging = false
```

### Common Issues

**"Failed to grab device: device or resource busy"**
- Another application may be using exclusive input access
- Try closing other input management tools

**"Emergency release triggered"**
- This is a safety feature - input was inactive for 30 seconds
- Increase timeout in code if needed (will be configurable soon)

**Mouse clicks not working**
- Fixed in latest version - update if you're on an older build
- Check debug logs for button mapping issues

**Keyboard input showing wrong characters**
- Keymap synchronization is not yet implemented
- Both systems should use the same keyboard layout

## Architecture

```
┌─── SERVER (Physical Input) ────────────────────────────────────────┐
│                                                                    │
│      Linux Input Devices          Waymon Server Process            │
│      (/dev/input/event*)   ────>  (captures evdev events)          │
│                              │                                     │
└──────────────────────────────┼─────────────────────────────────────┘
                               │
                  Input Events (Protobuf) over
                    Encrypted SSH Tunnel
                               │
                               ▼
┌─── CLIENT (Remote Control) ────────────────────────────────────────┐
│                                                                    │
│      Waymon Client Process         Wayland Virtual Input           │
│      (receives events)      ────>  (injects mouse/keyboard)        │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

## Contributing

Contributions are welcome! Areas where help is needed:

- Testing on different Wayland compositors
- Security review
- Documentation improvements
- Bug reports and fixes
- Multiple monitor support
- TUI polish

## Related Projects

- [Synergy](https://symless.com/synergy) - Commercial cross-platform solution
- [Barrier](https://github.com/debauchee/barrier) - Open-source fork of Synergy
- [Input-leap](https://github.com/input-leap/input-leap) - Barrier fork with Wayland support
- [waynergy](https://github.com/r-c-f/waynergy) - Synergy client for Wayland
- [lan-mouse](https://github.com/feschber/lan-mouse) - Rust-based input sharing

## License

MIT License - see LICENSE file for details

## Acknowledgments

- Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for the TUI
- Uses [Wish](https://github.com/charmbracelet/wish) for SSH server
- Protocol Buffers for efficient serialization
- The Wayland community for protocol documentation