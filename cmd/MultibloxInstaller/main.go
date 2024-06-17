package main

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
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
	force := false
	if len(os.Args) > 1 {
		if slices.Contains(os.Args[1:], "/force") {
			force = true
		}
	}
	if !force {
		answer := app.Ask("Would you like to install Multiblox [Y/n]? ", "y", "n")
		if answer == "n" {
			return nil
		}
	}
	if !app.Admin() {
		err := app.RunSelfAsAdmin("/force")
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
	err = registry.DeleteKey(registry.CURRENT_USER, app.ConfigKey)
	if err == nil {
		fmt.Printf("Removed application registry key.\n")
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
	uninstallKey, _, err = registry.CreateKey(registry.CURRENT_USER, app.UninstallKey, registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("Error accessing uninstall key: %s\n", err)
		return err
	}
	defer uninstallKey.Close()
	err = uninstallKey.SetStringValue("DisplayName", "Multiblox")
	if err != nil {
		fmt.Printf("Error updating display name: %s\n", err)
		return err
	}
	err = uninstallKey.SetStringValue("DisplayVersion", app.Version)
	if err != nil {
		fmt.Printf("Error updating display version: %s\n", err)
		return err
	}
	err = uninstallKey.SetStringValue("DisplayIcon", filepath.Join(appDir, "icon.ico"))
	if err != nil {
		fmt.Printf("Error updating display icon: %s\n", err)
		return err
	}
	estimatedSize, err := app.EstimatedSize()
	if err != nil {
		fmt.Printf("Error fetching estimated size: %s\n", err)
		return err
	}
	err = uninstallKey.SetDWordValue("EstimatedSize", estimatedSize)
	if err != nil {
		fmt.Printf("Error updating estimated size: %s\n", err)
		return err
	}
	installDate := time.Now().Format(time.DateOnly)
	err = uninstallKey.SetStringValue("InstallDate", installDate)
	if err != nil {
		fmt.Printf("Error updating install date: %s\n", err)
		return err
	}
	err = uninstallKey.SetStringValue("InstallLocation", appDir)
	if err != nil {
		fmt.Printf("Error updating install location: %s\n", err)
		return err
	}
	err = uninstallKey.SetStringValue("Publisher", "Intelblox Foundation")
	if err != nil {
		fmt.Printf("Error updating publisher: %s\n", err)
		return err
	}
	mbExecPath := filepath.Join(appDir, "Multiblox.exe")
	uninstallString := fmt.Sprintf("%s uninstall", mbExecPath)
	err = uninstallKey.SetStringValue("UninstallString", uninstallString)
	if err != nil {
		fmt.Printf("Error updating uninstall string: %s\n", err)
		return err
	}
	err = uninstallKey.SetStringValue("URLInfoAbout", "https://intelblox.org/multiblox")
	if err != nil {
		fmt.Printf("Error updating url info: %s\n", err)
		return err
	}
	fmt.Println("Created uninstall key.")
	rbxKeyCmd := fmt.Sprintf("\"%s\" %%1", filepath.Join(appDir, "MultibloxPlayer.exe"))
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
	installClientCmd := exec.Command(mbExecPath, "reinstall")
	err = installClientCmd.Run()
	if err != nil {
		return err
	}
	fmt.Printf("Installed Roblox client.\n")
	fmt.Printf("Press enter to exit.")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
	return nil
}
func main() {
	err := install()
	if err != nil {
		fmt.Printf("Press enter to exit.\n")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
	}
}
