#!/bin/bash
set -e

echo "========================================="
echo " Installing SSH over WebSocket Gateway..."
echo "========================================="

if ! command -v go &> /dev/null; then
    echo "Error: 'go' is required to compile and install from source."
    echo "Please install Go from https://golang.org/dl/ and try again."
    exit 1
fi

export GOPATH=$(go env GOPATH)
export PATH=$PATH:$GOPATH/bin

echo "Fetching and compiling the latest version..."
go install github.com/kelvinzer0/ssh-over-websocket/cmd/ssh-gateway@latest

echo ""
echo "Installation complete!"
echo "The binary has been installed to: $GOPATH/bin/ssh-gateway"
echo ""
echo "Make sure your GOPATH/bin is in your PATH. If you cannot run 'ssh-gateway',"
echo "you can run it using its full path: $GOPATH/bin/ssh-gateway"
echo ""
echo "To set it up as a system service, run:"
echo "  sudo $GOPATH/bin/ssh-gateway -install -bind :8080"
echo "========================================="
