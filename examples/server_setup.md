# Waymon Server Setup

With the new changes, the Waymon server must be run with sudo and uses system-wide configuration.

## Key Changes

1. **Server requires sudo**: The server will refuse to start without sudo privileges
2. **System-wide config**: Server uses `/etc/waymon/waymon.toml` (no fallback to user configs)
3. **Predictable socket**: Server socket is always at `/tmp/waymon.sock` for all users to connect

## Setup Steps

### 1. Create system config directory
```bash
sudo mkdir -p /etc/waymon
```

### 2. Run server (will create default config if missing)
```bash
sudo waymon server
```

### 3. Edit system config if needed
```bash
sudo nano /etc/waymon/waymon.toml
```

### 4. Client connection
Clients can connect to the server running on the same machine via the predictable socket path at `/tmp/waymon.sock`.

## Example systemd service

Create `/etc/systemd/system/waymon-server.service`:

```ini
[Unit]
Description=Waymon Mouse/Keyboard Sharing Server
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/waymon server --no-tui
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable waymon-server
sudo systemctl start waymon-server
```

## Security Notes

- The server runs as root to access input devices via uinput
- SSH keys are stored in `/etc/waymon/` for system-wide management
- Socket at `/tmp/waymon.sock` has permissions 0666 to allow all users to send switch commands
- The socket only accepts IPC commands (switch/status), not input events