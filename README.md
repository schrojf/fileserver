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
sudo journalctl -u fileserver.service --since "