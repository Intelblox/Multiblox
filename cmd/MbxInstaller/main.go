package main

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/Intelblox/Multiblox/internal/app"

	"os"
	"path/filepath"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

//go:embed assets
var assetsFs embed.FS

func install() error {
	y := false
	if len(os.Args) > 1 {
		if slices.Contains(os.Args[1:], "/y") {
			y = true
		}
	}
	if !y {
		answer := app.Ask("Would you like to install Multiblox (Y/n)? ", "y", "n")
		if answer == "n" {
			return nil
		}
	}
	if !app.Admin() {
		err := app.RunSelfAsAdmin("/y")
		if err != nil {
			fmt.Printf("To create URI protocols, administrative privileges are required. Install cannot proceed otherwise.\n")
		}
		return err
	}
	appDir, err := app.Directory()
	if err != nil {
		return err
	}
	processes, err := process.Processes()
	if err != nil {
		return err
	}
	_, err = os.Stat(appDir)
	if err == nil {
		for _, proc := range processes {
			execPath, err := proc.Exe()
			if err != nil {
				continue
			}
			if !strings.HasPrefix(execPath, appDir) {
				continue
			}
			err = proc.Kill()
			if err != nil {
				fmt.Printf("Error killing process occupying application directory: %s\n", err)
				return err
			}
			fmt.Printf("Killed process occupying application directory.\n")
		}
		time.Sleep(time.Second)
		err := os.RemoveAll(appDir)
		if err != nil {
			fmt.Printf("Error removing existing application directory: %s\n", err)
			return err
		}
		fmt.Println("Removed existing application directory.")
	}
	err = fs.WalkDir(assetsFs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		installPath := filepath.Join(appDir, strings.TrimPrefix(path, "assets/"))
		installDir := filepath.Dir(installPath)
		err = os.MkdirAll(installDir, os.ModeDir)
		if err != nil {
			return err
		}
		origin, err := assetsFs.Open(path)
		if err != nil {
			return err
		}
		dest, err := os.Create(installPath)
		if err != nil {
			return err
		}
		_, err = io.Copy(dest, origin)
		origin.Close()
		dest.Close()
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Error copying assets: %s\n", err)
		return err
	}
	fmt.Printf("Copied assets into application directory.\n")
	err = registry.DeleteKey(registry.CURRENT_USER, app.ConfigKey)
	if err == nil {
		fmt.Printf("Removed application key.\n")
	}
	appKey, _, err := registry.CreateKey(registry.CURRENT_USER, app.ConfigKey, registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("Error creating application key: %s\n", err)
		return err
	}
	appKeyValues := map[string]any{
		"UpdateNotifications":         uint32(1),
		"UpdateNotificationFrequency": uint64(60 * 60 * 24 * 2),
		"LastUpdateNotification":      uint64(0),
		"MultiInstancing":             uint32(1),
	}
	for name, value := range appKeyValues {
		var err error
		switch reflect.TypeOf(value).Kind() {
		case reflect.Uint32:
			err = appKey.SetDWordValue(name, value.(uint32))
		case reflect.Uint64:
			err = appKey.SetQWordValue(name, value.(uint64))
		}
		if err != nil {
			fmt.Printf("Error setting %s value: %s\n", name, err)
			return err
		}
	}
	fmt.Printf("Created application key.\n")
	uninstallKey, err := registry.OpenKey(registry.CURRENT_USER, app.UninstallKey, registry.ALL_ACCESS)
	if err == nil {
		uninstallKey.Close()
		err = registry.DeleteKey(registry.CURRENT_USER, app.UninstallKey)
		if err != nil {
			fmt.Printf("Error removing existing uninstall key: %s\n", err)
			return err
		}
		fmt.Printf("Removed existing uninstall key.\n")
	}
	estimatedSize, err := app.EstimatedSize()
	if err != nil {
		fmt.Printf("Error fetching estimated size: %s\n", err)
		return err
	}
	displayIconPath := filepath.Join(appDir, "icon.ico")
	installDate := time.Now().Format(time.DateOnly)
	mbxExePath := filepath.Join(appDir, "Mbx.exe")
	uninstallString := fmt.Sprintf("%s uninstall", mbxExePath)
	uninstallKeyValues := map[string]any{
		"DisplayName":     "Multiblox",
		"DisplayVersion":  app.Version,
		"DisplayIcon":     displayIconPath,
		"EstimatedSize":   estimatedSize,
		"InstallDate":     installDate,
		"InstallLocation": appDir,
		"Publisher":       "Intelblox Foundation",
		"UninstallString": uninstallString,
		"URLInfoAbout":    "https://intelblox.org/multiblox",
	}
	uninstallKey, _, err = registry.CreateKey(registry.CURRENT_USER, app.UninstallKey, registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("Error accessing uninstall key: %s\n", err)
		return err
	}
	defer uninstallKey.Close()
	for name, value := range uninstallKeyValues {
		var err error
		switch reflect.TypeOf(value).Kind() {
		case reflect.String:
			err = uninstallKey.SetStringValue(name, value.(string))
		case reflect.Uint32:
			err = uninstallKey.SetDWordValue(name, value.(uint32))
		}
		if err != nil {
			fmt.Printf("Error updating %s: %s\n", name, err)
			return err
		}
	}
	fmt.Println("Created uninstall key.")
	rbxKeyCmd := fmt.Sprintf("\"%s\" %%1", filepath.Join(appDir, "MbxPlayer.exe"))
	rbxKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, "roblox-player\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("Error accessing URI protocol: %s\n", err)
		return err
	}
	err = rbxKey.SetStringValue("", rbxKeyCmd)
	defer rbxKey.Close()
	if err != nil {
		fmt.Printf("Error updating URI protocol: %s\n", err)
		return err
	}
	sd, err := windows.SecurityDescriptorFromString("D:(A;OICI;GA;;;WD)")
	if err != nil {
		fmt.Printf("Error converting string to security descriptor: %s\n", err)
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		fmt.Printf("Error getting DACL: %s\n", err)
		return err
	}
	handle := windows.Handle(rbxKey)
	err = windows.SetSecurityInfo(handle, windows.SE_REGISTRY_KEY, windows.DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
	if err != nil {
		fmt.Printf("Error setting security info: %s\n", err)
		return err
	}
	fmt.Printf("Updated URI protocol.\n")
	envKey, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("Error accessing environment: %s\n", err)
		return err
	}
	defer envKey.Close()
	path, _, err := envKey.GetStringValue("Path")
	if err != nil {
		fmt.Printf("Error getting path: %s\n", err)
		return err
	}
	if !strings.Contains(path, appDir) {
		dirs := strings.Split(path, ";")
		dirs = append(dirs, appDir)
		path = strings.Join(dirs, ";")
		err = envKey.SetStringValue("Path", path)
		if err != nil {
			fmt.Printf("Error adding installation directory into PATH: %s\n", err)
			return err
		}
		fmt.Printf("Added installation directory into PATH.\n")
	}
	installClientCmd := exec.Command(mbxExePath, "install", "roblox")
	installClientCmd.Stdout = os.Stdout
	err = installClientCmd.Run()
	if err != nil {
		return err
	}
	fmt.Printf("Installed Roblox client.\n")
	if y {
		fmt.Printf("Press enter to exit.")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}
	return nil
}
func main() {
	err := install()
	if err != nil {
		fmt.Printf("Press enter to exit.\n")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}
}
