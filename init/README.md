# Waymon Systemd Service

This directory contains the systemd service file for running Waymon server as a system daemon.

## Installation

1. Copy the service file to systemd:
   ```bash
   sudo cp waymon.service /etc/systemd/system/
   ```

2. Install waymon binary:
   ```bash
   sudo cp /path/to/waymon /usr/local/bin/
   sudo chmod +x /usr/local/bin/waymon
   ```

3. Create configuration directory:
   ```bash
   sudo mkdir -p /etc/waymon
   ```

4. Generate SSH host key (if not already done):
   ```bash
   sudo ssh-keygen -t ed25519 -f /etc/waymon/ssh_host_key -N ""
   ```

5. Create authorized_keys file:
   ```bash
   sudo touch /etc/waymon/authorized_keys
   sudo chmod 600 /etc/waymon/authorized_keys
   ```

6. Enable and start the service:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable waymon.service
   sudo systemctl start waymon.service
   ```

## Usage

Once the service is running, you can control it using IPC commands as a regular user:

```bash
# Check server status
waymon status

# List connected clients
waymon list

# Switch to local control
waymon release

# Connect to a specific client (slot 1-5)
waymon connect 1

# Switch between clients
waymon switch next
waymon switch prev
waymon switch "client-name"
```

## Logs

View service logs:
```bash
sudo journalctl -u waymon -f
```

## Hyprland Integration

Add these keybindings to your Hyprland config:

```conf
# Waymon controls
bind = $mainMod ALT, 0, exec, waymon release
bind = $mainMod ALT, 1, exec, waymon connect 1
bind = $mainMod ALT, 2, exec, waymon connect 2
bind = $mainMod ALT, 3, exec, waymon connect 3
bind = $mainMod ALT, 4, exec, waymon connect 4
bind = $mainMod ALT, 5, exec, waymon connect 5
bind = $mainMod ALT, Tab, exec, waymon switch next
```

## Security

The service runs as root (required for uinput access) but includes security hardening:
- Restricted file system access
- Limited capabilities (only CAP_SYS_ADMIN for uinput)
- Network namespace restrictions
- Memory protections

IPC commands can be run by any user since the socket is created with appropriate permissions.