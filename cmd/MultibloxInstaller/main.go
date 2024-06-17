package main

import (
	"app"
	"bufio"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"strings"
	"time"

	"os"
	"path/filepath"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows/registry"
)

//go:embed assets
var assetsFs embed.FS

func install() error {
	fmt.Printf("Would you like to install Multiblox? Press enter to continue.")
	bufio.NewReader(os.Stdin).ReadBytes('\n')
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
		fmt.Println("Removed application registry.")
	}
	fmt.Printf("Configuration key: %s\n", app.ConfigKey)
	fmt.Printf("Application directory: %s\n", appDir)
	_, err = os.Stat(appDir)
	if err == nil {
		fmt.Println("Removing existing application directory.")
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
				return err
			}
		}
		err := os.RemoveAll(appDir)
		if err != nil {
			return err
		}
	}
	fmt.Println("Copying assets into application directory.")
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
		fmt.Printf("Error copying assets: %s", err)
		return err
	}
	fmt.Printf("Uninstall key: %s\n", app.UninstallKey)
	uninstallk, err := registry.OpenKey(registry.CURRENT_USER, app.UninstallKey, registry.ALL_ACCESS)
	if err == nil {
		uninstallk.Close()
		fmt.Println("Removing existing uninstall key.")
		err = registry.DeleteKey(registry.CURRENT_USER, app.UninstallKey)
		if err != nil {
			return err
		}
	}
	fmt.Println("Creating uninstall key.")
	uninstallk, _, err = registry.CreateKey(registry.CURRENT_USER, app.UninstallKey, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer uninstallk.Close()
	err = uninstallk.SetStringValue("DisplayName", "Multiblox")
	if err != nil {
		return err
	}
	err = uninstallk.SetStringValue("DisplayVersion", app.Version)
	if err != nil {
		return err
	}
	estimatedSize, err := app.EstimatedSize()
	if err != nil {
		return err
	}
	err = uninstallk.SetDWordValue("EstimatedSize", estimatedSize)
	if err != nil {
		return err
	}
	installDate := time.Now().Format(time.DateOnly)
	err = uninstallk.SetStringValue("InstallDate", installDate)
	if err != nil {
		return err
	}
	err = uninstallk.SetStringValue("InstallLocation", appDir)
	if err != nil {
		return err
	}
	err = uninstallk.SetStringValue("Publisher", "Intelblox Foundation")
	if err != nil {
		return err
	}
	mbExecPath := filepath.Join(appDir, "Multiblox.exe")
	uninstallString := fmt.Sprintf("%s uninstall", mbExecPath)
	err = uninstallk.SetStringValue("UninstallString", uninstallString)
	if err != nil {
		return err
	}
	err = uninstallk.SetStringValue("URLInfoAbout", "https://intelblox.org/multiblox")
	if err != nil {
		return err
	}
	fmt.Println("Modifying Roblox URI protocol.")
	rblxKeyCmd := fmt.Sprintf("\"%s\" %%1", filepath.Join(appDir, "MultibloxPlayer.exe"))
	rblxKey, err := registry.OpenKey(registry.CLASSES_ROOT, "roblox-player\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	err = rblxKey.SetStringValue("", rblxKeyCmd)
	rblxKey.Close()
	if err != nil {
		return err
	}
	rblxKey, err = registry.OpenKey(registry.CLASSES_ROOT, "roblox\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	err = rblxKey.SetStringValue("", rblxKeyCmd)
	rblxKey.Close()
	if err != nil {
		return err
	}
	envk, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer envk.Close()
	path, _, err := envk.GetStringValue("Path")
	if err != nil {
		return err
	}
	if !strings.Contains(path, appDir) {
		fmt.Println("Adding installation directory into PATH.")
		dirs := strings.Split(path, ";")
		dirs = append(dirs, appDir)
		path = strings.Join(dirs, ";")
		envk.SetStringValue("Path", path)
	}
	fmt.Printf("Installing Roblox client automatically.\n")
	installClientCmd := exec.Command(mbExecPath, "reinstall")
	err = installClientCmd.Run()
	if err != nil {
		return err
	}
	return nil
}
func main() {
	err := install()
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}
