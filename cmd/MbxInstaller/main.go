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
	"github.com/Intelblox/Multiblox/internal/procutil"
	"github.com/Intelblox/Multiblox/internal/reg"

	"os"
	"path/filepath"

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
		return fmt.Errorf("failed to run as admin: %s", err)
	}
	appDir, err := app.Directory()
	if err != nil {
		return err
	}
	_, err = os.Stat(appDir)
	if err == nil {
		err = procutil.ReleaseFileHandles(appDir)
		if err != nil {
			return fmt.Errorf("failed to release file handles: %s", err)
		}
		err := os.RemoveAll(appDir)
		if err != nil {
			return fmt.Errorf("failed to remove app directory: %s", err)
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
			return fmt.Errorf("failed to make install directory: %s", err)
		}
		origin, err := assetsFs.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open asset: %s", err)
		}
		dest, err := os.Create(installPath)
		if err != nil {
			return fmt.Errorf("failed to create asset file: %s", err)
		}
		_, err = io.Copy(dest, origin)
		origin.Close()
		dest.Close()
		if err != nil {
			return fmt.Errorf("failed to copy data from origin to destination: %s", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to copy assets: %s", err)
	}
	fmt.Printf("Copied assets into application directory.\n")
	err = reg.DeleteKey(registry.CURRENT_USER, app.ConfigKey)
	if err == nil {
		fmt.Printf("Removed application key.\n")
	}
	appKey, _, err := registry.CreateKey(registry.CURRENT_USER, app.ConfigKey, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to create app key: %s", err)
	}
	appKeyValues := map[string]any{
		"UpdateNotifications":         uint32(1),
		"UpdateNotificationFrequency": uint64(60 * 60 * 24 * 2),
		"LastUpdateNotification":      uint64(0),
		"MultiInstancing":             uint32(1),
		"DiscordRichPresence":         uint32(1),
		"DiscordShowJoinServer":       uint32(1),
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
			return fmt.Errorf("failed to set %s value: %s", name, err)
		}
	}
	fmt.Printf("Created application key.\n")
	uninstallKey, err := registry.OpenKey(registry.CURRENT_USER, app.UninstallKey, registry.ALL_ACCESS)
	if err == nil {
		uninstallKey.Close()
		err = reg.DeleteKey(registry.CURRENT_USER, app.UninstallKey)
		if err != nil {
			return fmt.Errorf("failed to delete uninstall key: %s", err)
		}
		fmt.Printf("Deleted existing uninstall key.\n")
	}
	estimatedSize, err := app.EstimatedSize()
	if err != nil {
		return fmt.Errorf("failed to get estimated size: %s", err)
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
		return fmt.Errorf("failed to create uninstall key: %s", err)
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
			return fmt.Errorf("failed to set %s value: %s", name, err)
		}
	}
	fmt.Println("Created uninstall key.")
	rbxKeyCmd := fmt.Sprintf("\"%s\" %%1", filepath.Join(appDir, "MbxPlayer.exe"))
	sd, err := windows.SecurityDescriptorFromString("D:(A;OICI;GA;;;WD)")
	if err != nil {
		return fmt.Errorf("failed to convert string to security descriptor: %s", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("failed to get DACL: %s", err)

	}
	for _, name := range []string{"roblox-player", "roblox"} {
		rbxKey, _, err := registry.CreateKey(registry.CLASSES_ROOT, name, registry.ALL_ACCESS)
		if err != nil {
			return fmt.Errorf("failed to create %s key: %s", name, err)
		}
		defer rbxKey.Close()
		err = rbxKey.SetStringValue("", "URL: Roblox Protocol")
		if err != nil {
			return fmt.Errorf("failed to set %s value: %s", name, err)
		}
		err = rbxKey.SetStringValue("URL Protocol", "")
		if err != nil {
			return fmt.Errorf("failed to set %s value: %s", name, err)
		}
		err = windows.SetSecurityInfo(windows.Handle(rbxKey), windows.SE_REGISTRY_KEY, windows.DACL_SECURITY_INFORMATION, nil, nil, dacl, nil)
		if err != nil {
			return fmt.Errorf("failed to set security info: %s", err)
		}
		rbxKey.Close()
		rbxKey, _, err = registry.CreateKey(registry.CLASSES_ROOT, fmt.Sprintf("%s\\shell\\open\\command", name), registry.ALL_ACCESS)
		if err != nil {
			return fmt.Errorf("failed to create command key: %s", err)
		}
		defer rbxKey.Close()
		err = rbxKey.SetStringValue("", rbxKeyCmd)
		if err != nil {
			return fmt.Errorf("failed to set command key value: %s", err)
		}
		rbxKey.Close()
	}
	fmt.Printf("Updated URI protocol.\n")
	envKey, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to open environment key: %s", err)
	}
	defer envKey.Close()
	path, _, err := envKey.GetStringValue("Path")
	if err != nil {
		return fmt.Errorf("failed to get PATH value: %s", err)
	}
	if !strings.Contains(path, appDir) {
		dirs := strings.Split(path, ";")
		dirs = append(dirs, appDir)
		path = strings.Join(dirs, ";")
		err = envKey.SetStringValue("Path", path)
		if err != nil {
			return fmt.Errorf("failed to add app directory into PATH: %s", err)
		}
		fmt.Printf("Added installation directory into PATH.\n")
	}
	envKey.Close()
	installClientCmd := exec.Command(mbxExePath, "install", "roblox")
	installClientCmd.Stdout = os.Stdout
	err = installClientCmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run roblox install command: %s", err)
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
		fmt.Printf("Error: %s\n", err)
		fmt.Printf("Press enter to exit.\n")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}
}
