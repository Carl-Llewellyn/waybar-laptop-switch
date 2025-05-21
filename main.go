package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"gitlab.com/UrsusArcTech/logger"
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

	var lastConfig string     // The last known config ("internal", "external", or "")
	var stableCount int       // Consecutive stable checks for the same config
	const stableThreshold = 3 // Number of consecutive checks required
	logger.LogMessage("Started")

	for range ticker.C {
		monitors, err := getMonitors()
		if err != nil {
			logger.LogError(fmt.Sprintf("Error retrieving monitors: %v", err))
			continue
		}

		descriptions := make([]string, 0, len(monitors))
		for _, m := range monitors {
			descriptions = append(descriptions, strings.TrimSpace(m.Description))
		}

		var currentConfig string
		var configPath string

		// When there's only one monitor, assume internal.
		if len(descriptions) == 1 && descriptions[0] == internalDesc {
			currentConfig = "internal"
			configPath = "/home/carl/.config/waybar/config-internal-monitor"
		} else if len(descriptions) == 4 {
			// External configuration applies only when exactly 4 displays are detected.
			// You may add further checks to verify that at least one external monitor matches.
			for _, desc := range descriptions {
				if desc == externalDesc {
					currentConfig = "external"
					configPath = "/home/carl/.config/waybar/config-ext-monitor"
					break
				}
			}
		}

		// If we don't recognize a valid configuration, reset debouncing and continue.
		if currentConfig == "" {
			stableCount = 0
			continue
		}

		// Increase our stability counter if the configuration hasn't changed.
		if currentConfig == lastConfig {
			stableCount++
		} else {
			stableCount = 1
		}

		// Only act if the configuration is stable for the prescribed threshold and it is a change.
		if stableCount >= stableThreshold && currentConfig != lastConfig {
			logger.LogMessage(fmt.Sprintf("Configuration change detected: %s (previous: %s). Restarting waybar...", currentConfig, lastConfig))
			if err := restartWaybar(configPath); err != nil {
				logger.LogError(fmt.Sprintf("Error restarting waybar with %s config: %v", currentConfig, err))
			} else {
				lastConfig = currentConfig
				logger.LogMessage(fmt.Sprintf("Waybar restarted with %s configuration.", currentConfig))
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

// restartWaybar kills any existing waybar process (that is not our child)
// and starts a new one with the provided config. It uses "pkill" to match the process name exactly.
func restartWaybar(configPath string) error {
	// Kill any running waybar process.
	killCmd := exec.Command("pkill", "-x", "waybar")
	if err := killCmd.Run(); err != nil {
		logger.LogWarning(fmt.Sprintf("Warning: pkill waybar resulted in an error (it might not be running): %v", err))
	}

	// Start waybar with the specified configuration, detached from this process.
	startCmd := exec.Command("waybar", "--config", configPath)
	startCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := startCmd.Start(); err != nil {
		return fmt.Errorf("failed to start waybar: %w", err)
	}

	logger.LogMessage(fmt.Sprintf("Started waybar (PID %d) with config %s", startCmd.Process.Pid, configPath))
	return nil
}
