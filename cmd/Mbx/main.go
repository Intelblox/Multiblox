package main

import (
	_ "embed"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
	"syscall"

	"os"
	"path/filepath"

	"github.com/Intelblox/Multiblox/internal/app"
	"github.com/Intelblox/Multiblox/internal/procutil"
	"github.com/Intelblox/Multiblox/internal/rbxapi"
	"github.com/Intelblox/Multiblox/internal/reg"
	"github.com/fatih/color"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func Config() error {
	appKey, err := registry.OpenKey(registry.CURRENT_USER, app.ConfigKey, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to open app key: %s", err)
	}
	names, err := appKey.ReadValueNames(0)
	if err != nil {
		return fmt.Errorf("failed to read value names: %s", err)
	}
	var queryName string
	var electedValue string
	var value uint32
	if len(os.Args) > 2 {
		queryName = os.Args[2]
		if len(os.Args) > 3 {
			electedValue = strings.ToLower(os.Args[3])
		}
	}
	if electedValue != "" && slices.Contains(names, queryName) {
		if slices.Contains([]string{"1", "on", "enable", "enabled", "true", "yes"}, electedValue) {
			value = 1
		} else if slices.Contains([]string{"0", "off", "disable", "disabled", "false", "no"}, electedValue) {
			value = 0
		}
		err = appKey.SetDWordValue(queryName, value)
		if err != nil {
			return fmt.Errorf("failed to set value of key: %s", err)
		}
	}
	for _, name := range names {
		if queryName != "" && queryName != name {
			continue
		}
		value, valType, err := appKey.GetIntegerValue(name)
		if valType != registry.DWORD {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to access value: %s", err)
		}
		valOut := color.RedString("off")
		if value == 1 {
			valOut = color.GreenString("on")
		}
		fmt.Fprintf(color.Output, "%s: %s\n", name, valOut)
	}
	return nil
}

func Help() error {
	appDir, err := app.Directory()
	if err != nil {
		return fmt.Errorf("failed to get app directory: %s", err)
	}
	commandsPath := filepath.Join(appDir, "commands.txt")
	data, err := os.ReadFile(commandsPath)
	if err != nil {
		return fmt.Errorf("failed to read commands file: %s", err)
	}
	fmt.Print(string(data))
	return nil
}

func Install() error {
	update := []string{}
	if len(os.Args) > 2 {
		update = append(update, os.Args[2:]...)
	}
	if len(update) == 0 {
		update = []string{"multiblox", "roblox"}
	}
	foundPackage := false
	if slices.Contains(update, "multiblox") {
		foundPackage = true
		err := InstallMultiblox(nil)
		if err != nil {
			err = fmt.Errorf("failed to install Multiblox: %s", err)
		}
		return err
	}
	if slices.Contains(update, "roblox") {
		foundPackage = true
		cvu, err := rbxapi.ClientVersionUpload(rbxapi.WindowsBinaryType, rbxapi.LiveChannel)
		if err != nil {
			return fmt.Errorf("failed to get latest Roblox version number: %s", err)
		}
		err = InstallRobloxClient(cvu)
		if err != nil {
			return fmt.Errorf("failed to install Roblox client: %s", err)
		}
	}
	if !foundPackage {
		return fmt.Errorf("failed to find package with name specified")
	}
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

func Uninstall() error {
	if !app.Admin() {
		err := app.RunSelfAsAdmin()
		if err != nil {
			fmt.Printf("To uninstall URI protocols, administrative privileges are required. Uninstall cannot proceed otherwise.\n")
			return fmt.Errorf("failed to run as admin: %s", err)
		}
		return nil
	}
	appDir, err := app.Directory()
	if err != nil {
		return fmt.Errorf("failed to get app directory: %s", err)
	}
	envk, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to open environment key: %s", err)
	}
	defer envk.Close()
	path, _, err := envk.GetStringValue("Path")
	if err != nil {
		return fmt.Errorf("failed to get PATH value: %s", err)
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
			return fmt.Errorf("failed to set PATH value: %s", err)
		}
		fmt.Printf("Removed Multiblox from PATH.\n")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %s", err)
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
		for _, name := range []string{"roblox", "roblox-player"} {
			err = reg.DeleteKey(registry.CLASSES_ROOT, name)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete %s protocol: %s", name, err)
			}
		}
		fmt.Printf("Deleted Roblox URI protocol.\n")
	} else {
		for _, name := range []string{"roblox\\shell\\open\\command", "roblox-player\\shell\\open\\command"} {
			rbxKey, err := registry.OpenKey(registry.CLASSES_ROOT, name, registry.ALL_ACCESS)
			if err != nil {
				return fmt.Errorf("failed to open %s key: %s", name, err)
			}
			defer rbxKey.Close()
			err = rbxKey.SetStringValue("", cmdString)
			if err != nil {
				return fmt.Errorf("failed to restore %s key: %s", name, err)
			}
			rbxKey.Close()
		}
		fmt.Printf("Restored Roblox URI protocol to default.\n")
	}
	err = reg.DeleteKey(registry.CURRENT_USER, app.UninstallKey)
	if err != nil {
		return fmt.Errorf("failed to delete uninstall key: %s", err)
	}
	fmt.Printf("Removed uninstall key.\n")
	uninstallOriginPath := filepath.Join(appDir, "uninstall.bat")
	uninstallOrigin, err := os.Open(uninstallOriginPath)
	if err != nil {
		return fmt.Errorf("failed to open uninstall script: %s", err)
	}
	defer uninstallOrigin.Close()
	uninstallPath := filepath.Join(os.TempDir(), "multiblox-uninstall.bat")
	uninstallF, err := os.Create(uninstallPath)
	if err != nil {
		return fmt.Errorf("failed to create copy of uninstall script: %s", err)
	}
	defer uninstallF.Close()
	_, err = io.Copy(uninstallF, uninstallOrigin)
	if err != nil {
		return fmt.Errorf("failed to copy uninstall script: %s", err)
	}
	uninstallOrigin.Close()
	uninstallF.Close()
	fmt.Printf("Copied uninstall script into temp directory.\n")
	err = procutil.ReleaseFileHandles(appDir)
	if err != nil {
		return fmt.Errorf("failed to release file handles: %s", err)
	}

	cmd := exec.Command(uninstallPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start uninstall script: %s", err)
	}
	fmt.Println("Ran uninstall script as a detached process.\n", err)
	return nil
}

func Update() error {
	y := false
	update := []string{}
	if len(os.Args) > 1 {
		if slices.Contains(os.Args[1:], "multiblox") {
			update = append(update, "multiblox")
		}
		if slices.Contains(os.Args[1:], "roblox") {
			update = append(update, "roblox")
		}
		if slices.Contains(os.Args[1:], "/y") {
			y = true
		}
	}
	if len(update) == 0 {
		update = []string{"multiblox", "roblox"}
	}
	updateMultiblox := slices.Contains(update, "multiblox")
	updateRoblox := slices.Contains(update, "roblox")
	if updateMultiblox {
		release, err := GetLatestRelease()
		if err != nil {
			return fmt.Errorf("failed to get latest Multiblox release: %s", err)
		}
		install := false
		latestMbxVersion := strings.TrimPrefix(release.TagName, "v")
		if latestMbxVersion != app.Version {
			fmt.Printf("Multiblox has an update.\n")
			fmt.Printf("Current version: %s\n", app.Version)
			fmt.Printf("Latest version: %s\n", latestMbxVersion)
			fmt.Printf("Release notes: %s\n", release.HtmlUrl)
			if !y {
				answer := app.Ask("Would you like to update (Y/n)? ", "y", "n")
				if answer == "y" {
					install = true
				}
			}
		} else {
			fmt.Printf("Your Multiblox client is up to date.\n")
		}
		if install {
			installer, err := GetInstaller(release)
			if err != nil {
				return fmt.Errorf("failed to get installer: %s", err)
			}
			err = InstallMultiblox(installer)
			if err != nil {
				return fmt.Errorf("failed to install Multiblox: %s", err)
			}
			return nil
		}
	}
	if updateRoblox {
		currentRbxVersion, err := reg.Get("RobloxClientVersion")
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		latestRbxVersion, err := rbxapi.ClientVersionUpload(rbxapi.WindowsBinaryType, rbxapi.LiveChannel)
		if err != nil {
			return err
		}
		if currentRbxVersion == latestRbxVersion {
			fmt.Printf("Your Roblox client is up to date.")
			return nil
		}
		fmt.Printf("Roblox has an update.\n")
		fmt.Printf("Current version: %s\n", currentRbxVersion)
		fmt.Printf("Latest version: %s\n", latestRbxVersion)
		install := false
		if !y {
			answer := app.Ask("Would you like to update (Y/n)? ", "y", "n")
			if answer == "y" {
				install = true
			}
		}
		if install {
			err = InstallRobloxClient(latestRbxVersion)
			if err != nil {
				return fmt.Errorf("failed to install Roblox: %s", err)
			}
		}
	}
	return nil
}

func Version() error {
	currentRbxVersion, err := reg.Get("RobloxClientVersion")
	fmt.Printf("Multiblox v%s\n", app.Version)
	if err == nil {
		fmt.Printf("Roblox %s\n", currentRbxVersion)
	}
	return nil
}

func main() {
	cmd := Help
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			cmd = Config
		case "help":
			cmd = Help
		case "install":
			cmd = Install
		case "logs":
			cmd = Logs
		case "uninstall":
			cmd = Uninstall
		case "update":
			cmd = Update
		case "version":
			cmd = Version
		}
	}
	err := cmd()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}
}
