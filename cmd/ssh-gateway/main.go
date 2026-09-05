package main

import (
	"flag"
	"fmt"
	"log"

	"ssh-over-websocket/internal/gateway"
	"ssh-over-websocket/internal/systemd"
)

func main() {
	bind := flag.String("bind", ":8080", "Bind address (e.g., 127.0.0.1:8080 or :8080)")
	install := flag.Bool("install", false, "Install as a Linux systemd service")
	flag.Parse()

	if *install {
		if err := systemd.InstallService(*bind); err != nil {
			log.Fatalf("Install failed (make sure you run as root): %v", err)
		}
		fmt.Println("Service installed and started successfully. You can check status with: systemctl status ssh-gateway")
		return
	}

	server := gateway.NewServer(*bind)
	if err := server.Run(); err != nil {
		log.Fatal(err)
	}
}
