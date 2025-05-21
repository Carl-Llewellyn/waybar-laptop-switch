package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Monitor represents one monitor as returned by hyprctl monitors -j.
type Monitor struct {
	Description string `json:"description"`
}

const (
	internalDesc = "Chimei Innolux Corporation 0x1777"
	externalDesc = "Dell Inc. DELL U2419H 2MSF7R2"
)

func main() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastConfig string        // last configuration type ("internal", "external" or "")
	var lastChangeTime time.Time // the time when the configuration last changed

	for range ticker.C {
		monitors, err := getMonitors()
		if err != nil {
			fmt.Println("Error retrieving monitors:", err)
			continue
		}

		descriptions := make([]string, 0, len(monitors))
		for _, m := range monitors {
			descriptions = append(descriptions, strings.TrimSpace(m.Description))
		}

		// Determine current configuration
		var currentConfig string
		var configPath string

		// If there is exactly one monitor and it is the internal monitor.
		if len(descriptions) == 1 && descriptions[0] == internalDesc {
			currentConfig = "internal"
			configPath = "/home/carl/.config/waybar/config-internal-monitor"
		} else if len(descriptions) == 4 {
			// Look for at least one external/dell monitor among the monitors.
			for _, desc := range descriptions {
				if desc == externalDesc {
					currentConfig = "external"
					configPath = "/home/carl/.config/waybar/config-ext-monitor"
					break
				}
			}
		}

		// If we did not recognize a valid configuration, skip this iteration.
		if currentConfig == "" {
			continue
		}

		// Only take action if the configuration has changed.
		if currentConfig != lastConfig {
			fmt.Printf("Configuration change detected: %s (previous: %s). Restarting waybar...\n", currentConfig, lastConfig)
			if err := restartWaybar(configPath); err != nil {
				fmt.Printf("Error restarting waybar with %s config: %v\n", currentConfig, err)
			} else {
				lastConfig = currentConfig
				lastChangeTime = time.Now()
				fmt.Printf("Waybar restarted at %s with %s configuration.\n", lastChangeTime.Format(time.RFC1123), currentConfig)
			}
		}
	}
}

// getMonitors runs "hyprctl monitors -j" and decodes the JSON output.
func getMonitors() ([]Monitor, error) {
	cmd := exec.Command("hyprctl", "monitors", "-j")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var monitors []Monitor
	if err := json.Unmarshal(out, &monitors); err != nil {
		return nil, err
	}
	return monitors, nil
}

// restartWaybar kills any existing waybar process (that is not our child) and starts a new one with the provided config.
// It uses "pkill" with the "-x" flag to kill only processes named exactly "waybar" then launches a new instance detached from our process.
func restartWaybar(configPath string) error {
	// Kill any running waybar processes.
	killCmd := exec.Command("pkill", "-x", "waybar")
	if err := killCmd.Run(); err != nil {
		// Warn if pkill returns an error (it might happen if waybar is not running).
		fmt.Println("Warning: pkill waybar resulted in an error (it might not be running):", err)
	}

	// Start waybar with the desired configuration.
	startCmd := exec.Command("waybar", "--config", configPath)
	// Detach the process so it doesn't become our child.
	startCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := startCmd.Start(); err != nil {
		return fmt.Errorf("failed to start waybar: %w", err)
	}

	fmt.Printf("Started waybar (PID %d) with config %s\n", startCmd.Process.Pid, configPath)
	return nil
}
