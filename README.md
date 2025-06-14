# Waymon

**Wayland Mouse Over Network** - A client/server mouse sharing application for Wayland systems. It allows seamless mouse movement between two computers on a local network, working around Wayland's security restrictions by using the uinput kernel module.

## Features

- **Wayland-native**: Works **ONLY** with Wayland compositors
- **Secure by design**: Uses SSH for encrypted, authenticated connections
- **Whitelist authentication**: Interactive approval for new SSH keys
- **Edge detection**: Move mouse to screen edge to switch between computers
- **Multi-monitor support**: Automatic detection of monitor configuration
- **Simple TUI**: Clean terminal interface for monitoring connections

## Prerequisites

### System Requirements

- Linux system with Wayland compositor
- uinput kernel module (usually available by default)
- Sudo privileges on both server and client machines (required for uinput access)

### Dependencies

The following system packages may be required:
- `uinput-tools` (recommended)
- Development headers if building from source

## Installation

### Binary Releases

Download the latest release from [GitHub Releases](https://github.com/bnema/waymon/releases).

#### Arch Linux

```bash
# Download the package
wget https://github.com/bnema/waymon/releases/download/v0.3/waymon_0.3_linux_amd64.pkg.tar.zst

# Install with pacman
sudo pacman -U waymon_0.3_linux_amd64.pkg.tar.zst
```

#### Debian/Ubuntu

```bash
# Download the package
wget https://github.com/bnema/waymon/releases/download/v0.3/waymon_0.3_linux_amd64.deb

# Install with dpkg
sudo dpkg -i waymon_0.3_linux_amd64.deb
```

#### Fedora/RHEL/CentOS

```bash
# Download the package
wget https://github.com/bnema/waymon/releases/download/v0.3/waymon_0.3_linux_amd64.rpm

# Install with rpm
sudo rpm -i waymon_0.3_linux_amd64.rpm
```


#### Generic Linux (Tarball)

```bash
# Download the tarball
wget https://github.com/bnema/waymon/releases/download/v0.3/waymon_Linux_x86_64.tar.gz

# Extract
tar -xzf waymon_Linux_x86_64.tar.gz

# Move to PATH
sudo mv waymon /usr/local/bin/
```

### From Source

```bash
git clone https://github.com/bnema/waymon.git
cd waymon
go build -o waymon .
```

### Using Go Install

```bash
go install github.com/bnema/waymon@latest
```

## Quick Start

1. Install Waymon on both computers using one of the methods above
2. On the server (computer to be controlled):
   ```bash
   waymon setup              # Set up uinput permissions
   sudo waymon server        # Start server
   ```
3. On the client (computer you're controlling from):
   ```bash
   waymon setup              # Set up uinput permissions
   waymon client --host SERVER_IP:52525
   ```
4. Move your mouse to the edge of the screen to switch between computers!

## Setup

### 1. uinput Permissions

Before using Waymon, you need to set up secure uinput permissions **on both server and client machines**:

```bash
waymon setup
```

This command automatically:
- Creates a dedicated `waymon` group for secure access
- Adds your user to the waymon group  
- Creates a udev rule that only allows waymon group access to uinput
- Configures everything needed for secure operation

**Important**: 
- Run `waymon setup` on both computers (server and client)
- You must log out and back in after setup for group changes to take effect (To be determined)
- Both machines need uinput access since the client also handles mouse input capture

### 2. Configuration

Copy the example configuration file:

```bash
cp waymon.toml.example ~/.config/waymon/waymon.toml
```

Edit the configuration file to match your network setup:

```toml
[server]
port = 52525
bind_address = "0.0.0.0"
name = "my-desktop"

[client]
server_address = "192.168.1.100:52525"
edge_threshold = 5

[[hosts]]
name = "laptop"
address = "192.168.1.100:52525"
position = "left"
```

## Usage

### Server Mode

Run the server on the computer you want to control:

```bash
sudo waymon server
```

Options:
- `--port, -p`: Port to listen on (default: 52525)
- `--bind, -b`: Bind address (default: 0.0.0.0)

Example:
```bash
sudo waymon server --port 52525 --bind 192.168.1.100
```

**Note**: Server mode requires root privileges for uinput access.

### Client Mode

Run the client on the computer you want to control from:

```bash
waymon client --host 192.168.1.100:52525
```

Options:
- `--host, -H`: Server address (host:port)
- `--edge, -e`: Edge detection size in pixels (default: 5)
- `--name, -n`: Use named host from configuration

Examples:
```bash
# Connect to specific server
waymon client --host 192.168.1.100:52525

# Use configured host
waymon client --name laptop

# Adjust edge sensitivity
waymon client --host 192.168.1.100:52525 --edge 10
```

### Using Named Hosts

Configure hosts in your `waymon.toml` file:

```toml
[[hosts]]
name = "laptop"
address = "192.168.1.100:52525"
position = "left"

[[hosts]]
name = "workstation"
address = "192.168.1.101:52525"
position = "right"
```

Then connect using the name:
```bash
waymon client --name laptop
```

## Testing

### Check Monitor Configuration

View your current monitor setup:

```bash
# Human-readable format
waymon monitors

# JSON format (for scripts)
waymon monitors --json
```

### Test Input Functionality

Test uinput functionality (requires root):

```bash
sudo waymon test input
```

This will draw circles with the mouse to verify input injection works.

## SSH Authentication

Waymon uses SSH key-based authentication with a whitelist system:

1. **First Connection**: When a new client connects, the server will prompt you to approve the SSH key
2. **Interactive Approval**: You'll see the client's address and SSH key fingerprint
3. **Whitelist Storage**: Approved keys are saved to the configuration file
4. **Future Connections**: Whitelisted keys connect automatically without prompts

To disable whitelist-only mode (accept all SSH keys without approval):
```toml
[server]
ssh_whitelist_only = false
```

## Configuration

Waymon looks for configuration files in the following order:
1. `./waymon.toml` (current directory)
2. `~/.config/waymon/waymon.toml` (user config)
3. `/etc/waymon/waymon.toml` (system config)

### Server Configuration

```toml
[server]
port = 52525                        # Port to listen on
bind_address = "0.0.0.0"            # Bind address
name = "my-desktop"                 # Server name
max_clients = 1                     # Maximum simultaneous clients
ssh_whitelist_only = true           # Only allow whitelisted SSH keys
ssh_whitelist = []                  # List of allowed SSH key fingerprints
```

### Client Configuration

```toml
[client]
server_address = ""       # Default server address
auto_connect = false      # Auto-connect on startup
reconnect_delay = 5       # Reconnection delay in seconds
edge_threshold = 5        # Edge detection sensitivity
```

### Display Configuration

```toml
[display]
refresh_interval = 5      # Monitor refresh interval
backend = "auto"          # Display backend
cursor_tracking = true    # Enable cursor tracking
```

## Troubleshooting

### Permission Errors

If you get permission errors:

1. Make sure you're running the server with `sudo`
2. Check uinput permissions: `ls -la /dev/uinput`
3. Run the setup command: `waymon setup`
4. Add your user to the input group: `sudo usermod -a -G input $USER`

### Connection Issues

If client can't connect to server:

1. Check if server is running: `sudo waymon server`
2. Verify network connectivity: `ping <server-ip>`
3. Check firewall settings on server
4. Ensure correct port is configured (default: 52525)

### Display Detection Problems

If monitors aren't detected properly:

1. Check monitor detection: `waymon monitors`
2. Try different display backends in configuration
3. Check if running under Wayland compositor
4. Verify wlr-randr or similar tools work

### Input Not Working

If mouse input isn't being injected:

1. Test input functionality: `sudo waymon test input`
2. Check uinput module: `lsmod | grep uinput`
3. Load uinput module: `sudo modprobe uinput`
4. Verify /dev/uinput exists and is writable

## Architecture

Waymon uses a client/server architecture:

- **Server**: Receives mouse events and injects them via uinput
- **Client**: Captures mouse at screen edges and sends events to server
- **Protocol**: TCP connection with Protocol Buffers serialization
- **UI**: Bubble Tea terminal interface for monitoring

## License

MIT License - see LICENSE file for details.