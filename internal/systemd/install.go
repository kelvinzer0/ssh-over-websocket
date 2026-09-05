package systemd

import (
	"fmt"
	"os"
	"os/exec"
)

// InstallService installs the application as a systemd service on Linux
func InstallService(bindAddr, certFile, keyFile string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %v", err)
	}

	execArgs := fmt.Sprintf("-bind %s", bindAddr)
	if certFile != "" && keyFile != "" {
		execArgs += fmt.Sprintf(" -cert %s -key %s", certFile, keyFile)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=SSH over WebSocket Gateway
After=network.target

[Service]
ExecStart=%s %s
Restart=always
User=root

[Install]
WantedBy=multi-user.target
`, exePath, execArgs)

	servicePath := "/etc/systemd/system/ssh-gateway.service"
	err = os.WriteFile(servicePath, []byte(serviceContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write systemd service file: %v", err)
	}

	fmt.Println("Reloading systemd daemon...")
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("failed to reload daemon: %v", err)
	}

	fmt.Println("Enabling and starting ssh-gateway service...")
	if err := exec.Command("systemctl", "enable", "--now", "ssh-gateway").Run(); err != nil {
		return fmt.Errorf("failed to enable service: %v", err)
	}

	return nil
}
