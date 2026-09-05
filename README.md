# SSH over WebSocket Gateway

![Go Version](https://img.shields.io/badge/go-1.21%2B-blue.svg)
![Build Status](https://github.com/kelvinzer0/ssh-over-websocket/actions/workflows/ci.yml/badge.svg)

A simple, lightweight gateway to bridge SSH connections over WebSockets, complete with a web-based terminal frontend using [xterm.js](https://xtermjs.org/).

## Architecture

```text
[ Frontend / Xterm.js ]               [ Go SSH Gateway ]               [ Server SSH Target ]
          |                                   |                                   |
          | 1. Buka WebSocket dengan Query    |                                   |
          |    (/ssh?host=...&user=...&pass=.)|                                   |
          |---------------------------------->|                                   |
          |                                   | 2. Baca Query & Hubungkan TCP     |
          |                                   |---------------------------------->|
          |                                   | 3. Buka Terminal Virtual (PTY)    |
          |                                   |---------------------------------->|
          |                                   |                                   |
          |<========== 4. Jembatan WebSocket - SSH Terhubung Aktif ===========>   |
```

## Features

- **Go Backend**: Uses `golang.org/x/crypto/ssh` and `github.com/gorilla/websocket` for efficient connections.
- **Web Frontend**: Built-in simple UI leveraging `xterm.js` for a fully functional in-browser terminal experience.
- **Easy Deployment**: A single Go binary is all you need to serve the frontend and handle the WebSocket-SSH bridging.

## Getting Started

### Prerequisites
- Go 1.21 or newer.

### Installation

**Option 1: Quick Install via Curl**
You can install the latest version directly using our installation script (requires Go):
```bash
curl -sSL https://raw.githubusercontent.com/kelvinzer0/ssh-over-websocket/master/install.sh | bash
```

**Option 2: Using `go install`**
```bash
go install github.com/kelvinzer0/ssh-over-websocket/cmd/ssh-gateway@latest
```

**Option 3: Build from Source**
```bash
git clone https://github.com/kelvinzer0/ssh-over-websocket.git
cd ssh-over-websocket
go build -o ssh-gateway ./cmd/ssh-gateway
```

### Usage

Run the gateway on the default port (`:8080`):
```bash
ssh-over-websocket
```
Or specify a custom binding host and port using the `-bind` flag:
```bash
ssh-over-websocket -bind 127.0.0.1:9090
```

Open your browser and navigate to `http://localhost:9090`.
- **Host**: IP address or hostname of the target SSH server (e.g., `127.0.0.1`).
- **Port**: SSH Port (default is `22`).
- **Username**: SSH Username.
- **Password**: SSH Password.

Click **Connect SSH** and your terminal session will start within the browser.

### Running with HTTPS / SSL

To run the server securely over HTTPS, simply provide the path to your SSL certificate and key files:
```bash
ssh-over-websocket -bind 0.0.0.0:443 -cert /path/to/cert.pem -key /path/to/key.pem
```
The server will automatically switch from HTTP to HTTPS when both flags are provided.

### Auto Setup Linux Service (Systemd)

You can automatically configure the gateway to run as a background service on Linux. Run the binary as root with the `-install` flag. You can also combine it with the `-bind`, `-cert`, and `-key` flags to set the default service configuration.

```bash
sudo ssh-gateway -install -bind 0.0.0.0:443 -cert /etc/ssl/cert.pem -key /etc/ssl/key.pem
```
This command will create a systemd service (`/etc/systemd/system/ssh-gateway.service`), reload the daemon, and enable the service to start automatically on boot.

## CI/CD
This repository is configured with a GitHub Actions workflow (`ci.yml`) to automatically build and test the Go code on every push and pull request.

## License
MIT License
