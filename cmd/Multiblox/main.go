package main

import (
	_ "embed"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"

	"os"
	"path/filepath"

	"github.com/Intelblox/Multiblox/internal/app"
	"github.com/Intelblox/Multiblox/internal/rbxapi"
	"github.com/Intelblox/Multiblox/internal/reg"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func Help() error {
	fmt.Printf("USAGE: multiblox [help|version|update|reinstall|uninstall|logs]\n")
	return nil
}

func Version() error {
	fmt.Printf("Multiblox v%s\n", app.Version)
	return nil
}

func Update() error {
	rcv, err := reg.GetRobloxClientVersion()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if rcv != "" {
		fmt.Printf("Current ROBLOX client version: %s\n", rcv)
	}
	cvu, err := rbxapi.ClientVersionUpload(rbxapi.WindowsBinaryType, rbxapi.LiveChannel)
	if err != nil {
		return err
	}
	if rcv == cvu {
		fmt.Printf("You are currently up to date.")
		return nil
	} else {
		fmt.Printf("Latest ROBLOX client version: %s\n", cvu)
	}
	answer := app.Ask("Would you like to update to the latest version [Y/N]? ", "y", "n")
	if answer == "y" {
		InstallRobloxClient(cvu)
	}
	return nil
}

func Reinstall() error {
	cvu, err := rbxapi.ClientVersionUpload(rbxapi.WindowsBinaryType, rbxapi.LiveChannel)
	if err != nil {
		return err
	}
	InstallRobloxClient(cvu)
	return nil
}

func Uninstall() error {
	if !app.Admin() {
		err := app.RunSelfAsAdmin()
		if err != nil {
			fmt.Printf("To uninstall URI protocols, administrative privileges are required. Uninstall cannot proceed otherwise.\n")
		}
		return err
	}
	appDir, err := app.Directory()
	if err != nil {
		fmt.Printf("Error getting app directory: %s\n", err)
		return err
	}
	envk, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("Error accessing environment: %s\n", err)
		return err
	}
	defer envk.Close()
	path, _, err := envk.GetStringValue("Path")
	if err != nil {
		fmt.Printf("Error accessing PATH: %s\n", err)
		return err
	}
	if strings.Contains(path, appDir) {
		newDirs := []string{}
		dirs := strings.Split(path, ";")
		for _, dir := range dirs {
			if dir != appDir {
				newDirs = append(newDirs, dir)
			}
		}
		path = strings.Join(newDirs, ";")
		err = envk.SetStringValue("Path", path)
		if err != nil {
			fmt.Printf("Error removing Multiblox from PATH: %s", err)
			return err
		}
		fmt.Printf("Removed Multiblox from PATH.\n")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting home directory: %s\n", err)
		return err
	}
	rbxClientsDir := filepath.Join(home, "AppData", "Local", "Roblox", "Versions")
	versions, _ := os.ReadDir(rbxClientsDir)
	cmdString := ""
	for _, version := range versions {
		rbxClientPath := filepath.Join(rbxClientsDir, version.Name(), "RobloxPlayerBeta.exe")
		_, err := os.Stat(rbxClientPath)
		if err == nil {
			cmdString = rbxClientPath
		}
	}
	if cmdString == "" {
		err = registry.DeleteKey(registry.CLASSES_ROOT, "roblox-player\\shell\\open\\command")
		if err != nil {
			fmt.Printf("Error deleting Roblox URI protocol: %s\n", err)
			return err
		}
		fmt.Printf("Deleted Roblox URI protocol.\n")
	} else {
		rbxKey, err := registry.OpenKey(registry.CLASSES_ROOT, "roblox-player\\shell\\open\\command", registry.ALL_ACCESS)
		if err != nil {
			fmt.Printf("Error opening URI protocol key: %s\n", err)
			return err
		}
		defer rbxKey.Close()
		err = rbxKey.SetStringValue("", cmdString)
		if err != nil {
			fmt.Printf("Error restoring URI protocol to default: %s\n", err)
			return err
		}
		rbxKey.Close()
		fmt.Printf("Restored Roblox URI protocol to default.\n")
	}
	err = registry.DeleteKey(registry.CURRENT_USER, app.UninstallKey)
	if err != nil {
		fmt.Printf("Error removing uninstall key: %s\n", err)
		return err
	}
	fmt.Printf("Removed uninstall key.\n")
	uninstallOriginPath := filepath.Join(appDir, "uninstall.bat")
	uninstallOrigin, err := os.Open(uninstallOriginPath)
	if err != nil {
		fmt.Printf("Error opening uninstall batch: %s", err)
		return err
	}
	defer uninstallOrigin.Close()
	uninstallPath := filepath.Join(os.TempDir(), "multiblox-uninstall.bat")
	uninstallF, err := os.Create(uninstallPath)
	if err != nil {
		fmt.Printf("Error creating uninstall batch: %s", err)
		return err
	}
	defer uninstallF.Close()
	_, err = io.Copy(uninstallF, uninstallOrigin)
	if err != nil {
		fmt.Printf("Error copying uninstall batch: %s", err)
		return err
	}
	uninstallOrigin.Close()
	uninstallF.Close()
	fmt.Printf("Copied uninstall batch into temp directory.\n")
	processes, err := process.Processes()
	if err != nil {
		fmt.Printf("Error fetching processes: %s\n", err)
		return err
	}
	for _, proc := range processes {
		name, err := proc.Name()
		if err != nil {
			continue
		}
		if name != "RobloxPlayerBeta.exe" {
			continue
		}
		path, err := proc.Exe()
		if err != nil {
			continue
		}
		if !strings.HasPrefix(path, appDir) {
			continue
		}
		proc.Kill()
		fmt.Printf("Error killing RobloxPlayerBeta process: %s\n", err)
	}
	cmd := exec.Command(uninstallPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	err = cmd.Start()
	if err != nil {
		fmt.Printf("Error running uninstall.bat as a detached process: %s\n", err)
		return err
	}
	fmt.Println("Ran uninstall.bat as a detached process.\n", err)
	return nil
}

func Logs() error {
	appDir, err := app.Directory()
	if err != nil {
		return err
	}
	cmd := exec.Command("explorer.exe", filepath.Join(appDir, "Logs"))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	return nil
}

func main() {
	cmd := Help
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "help":
			cmd = Help
		case "version":
			cmd = Version
		case "update":
			cmd = Update
		case "reinstall":
			cmd = Reinstall
		case "uninstall":
			cmd = Uninstall
		case "logs":
			cmd = Logs
		}
	}
	cmd()
}
