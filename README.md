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

### Installation & Run

1. Clone the repository:
   ```bash
   git clone https://github.com/kelvinzer0/ssh-over-websocket.git
   cd ssh-over-websocket
   ```

2. Run the application:
   ```bash
   go run main.go
   ```

3. Open your browser and navigate to `http://localhost:8080`.

### Usage
- **Host**: IP address or hostname of the target SSH server (e.g., `127.0.0.1`).
- **Port**: SSH Port (default is `22`).
- **Username**: SSH Username.
- **Password**: SSH Password.

Click **Connect SSH** and your terminal session will start within the browser.

## CI/CD
This repository is configured with a GitHub Actions workflow (`ci.yml`) to automatically build and test the Go code on every push and pull request.

## License
MIT License
