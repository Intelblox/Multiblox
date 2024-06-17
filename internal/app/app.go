package app

import (
	"io/fs"
	"os"
	"path/filepath"
)

var Version = "1.0.0"
var UninstallKey = "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\Multiblox"
var ConfigKey = "SOFTWARE\\Intelblox Foundation\\Multiblox"

func Directory() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	installDir := filepath.Join(homeDir, "AppData", "Local", "Multiblox")
	return installDir, nil
}

func EstimatedSize() (uint32, error) {
	appDir, err := Directory()
	if err != nil {
		return 0, err
	}
	var estimatedSize uint32 = 0
	filepath.Walk(appDir, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			estimatedSize += uint32(info.Size())
		}
		return nil
	})
	estimatedSize = estimatedSize / 1000
	return estimatedSize, nil
}
