# Simple Web File Server - Installation Guide

A secure, lightweight file server written in Go with embedded templates, similar to Apache HTTP Server's file explorer functionality.

## Features

- 🔒 **Secure**: Path traversal protection, no external dependencies
- 🚀 **Fast**: Single static binary, embedded templates
- 📱 **Responsive**: Mobile-friendly HTML interface with modern design
- 🔧 **Cross-platform**: Works on Linux x64 and ARM (Raspberry Pi)
- 🎯 **Simple**: Easy installation and configuration

## Quick Start

### 1. Download or Build

**Option A: Build from source**
```bash
# Clone or create the project files
git clone https://github.com/schrojf/fileserver
cd fileserver

# Build for current platform
make build

# Or build for specific platform
make build-linux-amd64    # For x64 systems
make build-linux-arm64    # For Raspberry Pi 4
make build-linux-arm      # For Raspberry Pi 2/3
```

**Option B: Manual build**
```bash
# Create project structure
mkdir -p fileserver/templates
cd fileserver

# Create the files (main.go, go.mod, templates/directory.html)
# ... (copy the provided files)

# Build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o fileserver .
```

### 2. Test Run
```bash
# Test the binary
./fileserver -help
./fileserver -root /path/to/serve -port 8080

# Access via browser
# http://localhost:8080
```

## Installation on Debian/Ubuntu

### Method 1: System Service Installation (Recommended)

#### Step 1: Create User and Directories
```bash
# Create dedicated user for security
sudo useradd --system --shell /bin/false --home-dir /var/lib/fileserver fileserver

# Create directories
sudo mkdir -p /var/www/fileserver
sudo mkdir -p /var/lib/fileserver
sudo mkdir -p /var/log/fileserver

# Set permissions
sudo chown fileserver:fileserver /var/www/fileserver
sudo chown fileserver:fileserver /var/lib/fileserver
sudo chmod 755 /var/www/fileserver
```

#### Step 2: Install Binary
```bash
# Copy binary to system location
sudo cp fileserver /usr/local/bin/fileserver
sudo chmod +x /usr/local/bin/fileserver

# Verify installation
/usr/local/bin/fileserver -help
```

#### Step 3: Install System Service
```bash
# Copy service file
sudo cp fileserver.service /etc/systemd/system/

# Reload systemd and enable service
sudo systemctl daemon-reload
sudo systemctl enable fileserver.service

# Start the service
sudo systemctl start fileserver.service

# Check status
sudo systemctl status fileserver.service
```

#### Step 4: Configure Firewall (if needed)
```bash
# Allow HTTP traffic (adjust port as needed)
sudo ufw allow 8080/tcp

# For HTTPS (if you add TLS later)
sudo ufw allow 443/tcp
```

### Method 2: Manual Installation

#### Create Configuration
```bash
# Create config directory
sudo mkdir -p /etc/fileserver

# Create simple config file (optional)
sudo tee /etc/fileserver/config.env << EOF
FILESERVER_ROOT=/var/www/fileserver
FILESERVER_PORT=8080
EOF
```

#### Create Startup Script
```bash
sudo tee /etc/init.d/fileserver << 'EOF'
#!/bin/bash
### BEGIN INIT INFO
# Provides:          fileserver
# Required-Start:    $network $local_fs
# Required-Stop:     $network $local_fs
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: Simple Web File Server
### END INIT INFO

DAEMON=/usr/local/bin/fileserver
DAEMON_USER=fileserver
DAEMON_ARGS="-root /var/www/fileserver -port 8080"
PIDFILE=/var/run/fileserver.pid

case "$1" in
  start)
    echo "Starting fileserver..."
    start-stop-daemon --start --quiet --pidfile $PIDFILE \
      --make-pidfile --background --chuid $DAEMON_USER \
      --exec $DAEMON -- $DAEMON_ARGS
    ;;
  stop)
    echo "Stopping fileserver..."
    start-stop-daemon --stop --quiet --pidfile $PIDFILE
    rm -f $PIDFILE
    ;;
  restart)
    $0 stop
    sleep 1
    $0 start
    ;;
  status)
    if [ -f $PIDFILE ]; then
      echo "fileserver is running (PID $(cat $PIDFILE))"
    else
      echo "fileserver is not running"
    fi
    ;;
  *)
    echo "Usage: $0 {start|stop|restart|status}"
    exit 1
    ;;
esac
EOF

sudo chmod +x /etc/init.d/fileserver
sudo update-rc.d fileserver defaults
```

## Configuration Options

### Command Line Arguments
- `-root`: Root directory to serve (default: current directory)
- `-port`: Port to listen on (default: 8080)
- `-help`: Show help message

### Examples
```bash
# Serve current directory on port 8080
./fileserver

# Serve specific directory
./fileserver -root /home/user/documents

# Use custom port
./fileserver -root /var/www -port 9000

# Serve on privileged port (requires sudo or capabilities)
sudo ./fileserver -root /var/www -port 80
```

### Service Configuration

Edit `/etc/systemd/system/fileserver.service`:

```ini
[Service]
# Change root directory
ExecStart=/usr/local/bin/fileserver -root /path/to/your/files -port 8080

# Change port (remember to update firewall rules)
ExecStart=/usr/local/bin/fileserver -root /var/www/fileserver -port 9000

# Run as different user
User=www-data
Group=www-data
```

After changes:
```bash
sudo systemctl daemon-reload
sudo systemctl restart fileserver.service
```

## Security Considerations

### 1. File Permissions
```bash
# Ensure served files have appropriate permissions
sudo find /var/www/fileserver -type f -exec chmod 644 {} \;
sudo find /var/www/fileserver -type d -exec chmod 755 {} \;
```

### 2. Firewall Configuration
```bash
# Only allow necessary ports
sudo ufw allow from 192.168.1.0/24 to any port 8080  # Local network only
sudo ufw deny 8080  # Block external access
```

### 3. Reverse Proxy Setup (Nginx)
```nginx
server {
    listen 80;
    server_name your-domain.com;
    
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 4. SSL/TLS with Let's Encrypt
```bash
# Install certbot
sudo apt update
sudo apt install certbot python3-certbot-nginx

# Get certificate
sudo certbot --nginx -d your-domain.com

# Auto-renewal
sudo crontab -e
# Add: 0 12 * * * /usr/bin/certbot renew --quiet
```

## Monitoring and Logs

### View Service Logs
```bash
# View recent logs
sudo journalctl -u fileserver.service -f

# View logs since boot
sudo journalctl -u fileserver.service -b

# View logs for specific date
sudo journalctl -u fileserver.service --since "2024-01-01"
```

### Service Management Commands
```bash
# Start service
sudo systemctl start fileserver.service

# Stop service
sudo systemctl stop fileserver.service

# Restart service
sudo systemctl restart fileserver.service

# Check status
sudo systemctl status fileserver.service

# Enable auto-start at boot
sudo systemctl enable fileserver.service

# Disable auto-start
sudo systemctl disable fileserver.service
```

### Performance Monitoring
```bash
# Check resource usage
sudo systemctl status fileserver.service --lines=0 --no-pager
top -p $(pgrep fileserver)

# Monitor connections
sudo netstat -tulnp | grep :8080
sudo ss -tulnp | grep :8080
```

## Troubleshooting

### Common Issues

#### 1. Permission Denied
```bash
# Check file permissions
ls -la /usr/local/bin/fileserver
sudo chmod +x /usr/local/bin/fileserver

# Check service user permissions
sudo -u fileserver ls /var/www/fileserver
```

#### 2. Port Already in Use
```bash
# Find what's using the port
sudo netstat -tulnp | grep :8080
sudo lsof -i :8080

# Kill process if needed
sudo kill -9 <PID>
```

#### 3. Service Won't Start
```bash
# Check service status and logs
sudo systemctl status fileserver.service -l
sudo journalctl -u fileserver.service --no-pager

# Test binary manually
sudo -u fileserver /usr/local/bin/fileserver -root /var/www/fileserver -port 8080
```

#### 4. Can't Access from Network
```bash
# Check if service is listening on all interfaces
sudo netstat -tulnp | grep fileserver

# Check firewall
sudo ufw status verbose

# Test local connection
curl http://localhost:8080
```

### Debug Mode
```bash
# Run in foreground for debugging
sudo -u fileserver /usr/local/bin/fileserver -root /var/www/fileserver -port 8080

# Check system resources
free -h
df -h
```

## Updating

### Update Binary
```bash
# Stop service
sudo systemctl stop fileserver.service

# Backup current binary
sudo cp /usr/local/bin/fileserver /usr/local/bin/fileserver.backup

# Install new binary
sudo cp new-fileserver /usr/local/bin/fileserver
sudo chmod +x /usr/local/bin/fileserver

# Start service
sudo systemctl start fileserver.service

# Verify
sudo systemctl status fileserver.service
```

### Rollback
```bash
# Stop service
sudo systemctl stop fileserver.service

# Restore backup
sudo cp /usr/local/bin/fileserver.backup /usr/local/bin/fileserver

# Start service
sudo systemctl start fileserver.service
```

## Uninstallation

### Remove Service
```bash
# Stop and disable service
sudo systemctl stop fileserver.service
sudo systemctl disable fileserver.service

# Remove service file
sudo rm /etc/systemd/system/fileserver.service
sudo systemctl daemon-reload
```

### Remove Files
```bash
# Remove binary
sudo rm /usr/local/bin/fileserver

# Remove user and directories (optional)
sudo userdel fileserver
sudo rm -rf /var/lib/fileserver
sudo rm -rf /var/log/fileserver

# Keep or remove served files
# sudo rm -rf /var/www/fileserver
```

## Advanced Configuration

### Environment Variables
Create `/etc/systemd/system/fileserver.service.d/override.conf`:
```ini
[Service]
Environment=FILESERVER_ROOT=/custom/path
Environment=FILESERVER_PORT=9000
```

### Multiple Instances
```bash
# Copy service file for second instance
sudo cp /etc/systemd/system/fileserver.service /etc/systemd/system/fileserver2.service

# Edit the new service file
sudo systemctl edit fileserver2.service
```

```ini
[Service]
ExecStart=
ExecStart=/usr/local/bin/fileserver -root /var/www/fileserver2 -port 8081
```

### Custom Systemd Service Template
Create `/etc/systemd/system/fileserver@.service`:
```ini
[Unit]
Description=Simple Web File Server (%i)
After=network.target

[Service]
Type=simple
User=fileserver
Group=fileserver
ExecStart=/usr/local/bin/fileserver -root /var/www/fileserver-%i -port 808%i
Restart=always

[Install]
WantedBy=multi-user.target
```

Usage:
```bash
# Start multiple instances
sudo systemctl start fileserver@1.service  # Port 8081, /var/www/fileserver-1
sudo systemctl start fileserver@2.service  # Port 8082, /var/www/fileserver-2
```

## Performance Tuning

### System Limits
Edit `/etc/systemd/system/fileserver.service`:
```ini
[Service]
LimitNOFILE=65536
LimitNPROC=4096
LimitMEMLOCK=infinity
```

### Kernel Parameters
Add to `/etc/sysctl.conf`:
```
net.core.somaxconn = 65535
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_max_syn_backlog = 65535
```

Apply:
```bash
sudo sysctl -p
```

## Integration Examples

### Docker Integration
```dockerfile
FROM scratch
COPY fileserver /fileserver
COPY --from=alpine:latest /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
EXPOSE 8080
ENTRYPOINT ["/fileserver"]
CMD ["-root", "/data", "-port", "8080"]
```

### Nginx Reverse Proxy
```nginx
upstream fileserver {
    server 127.0.0.1:8080;
    keepalive 32;
}

server {
    listen 80;
    server_name files.example.com;
    
    location / {
        proxy_pass http://fileserver;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Optional: Add authentication
        auth_basic "Restricted Access";
        auth_basic_user_file /etc/nginx/.htpasswd;
    }
}
```

## Raspberry Pi Specific Notes

### Installation on Raspberry Pi OS
```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Go (if building from source)
sudo apt install golang-go

# Build for ARM
make build-linux-arm64  # For Pi 4
make build-linux-arm    # For Pi 2/3

# Follow standard installation steps
```

### Performance on Raspberry Pi
- Use ARM64 build on Raspberry Pi 4 for better performance
- Consider using external storage (USB 3.0) for better I/O
- Monitor temperature: `vcgencmd measure_temp`
- Adjust service limits for lower memory usage

### Auto-start on Boot
```bash
# Enable service
sudo systemctl enable fileserver.service

# Check boot time
systemd-analyze blame | grep fileserver
```

---

## Support

For issues and questions:
1. Check the logs: `sudo journalctl -u fileserver.service -f`
2. Verify configuration and permissions
3. Test with minimal setup
4. Check firewall and network settings

The file server is designed to be simple and reliable. Most issues are related to permissions, networking, or configuration rather than the application itself.