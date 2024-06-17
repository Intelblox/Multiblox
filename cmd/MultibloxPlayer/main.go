package main

import (
	_ "embed"
	"fmt"

	"github.com/Intelblox/Multiblox/internal/app"
	"github.com/Intelblox/Multiblox/internal/rbxapi"
	"github.com/Intelblox/Multiblox/internal/reg"

	"os/exec"
	"syscall"
	"time"

	"os"
	"path/filepath"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"gopkg.in/toast.v1"
)

func log(format string, message ...any) error {
	appDir, err := app.Directory()
	if err != nil {
		return err
	}
	dir := filepath.Join(appDir, "Logs")
	path := filepath.Join(dir, "player.log")
	err = os.MkdirAll(dir, os.ModeDir)
	if err != nil {
		return err
	}
	logf, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		return err
	}
	defer logf.Close()
	line := fmt.Sprintf(format, message...)
	line = fmt.Sprintf("[%s] %s\n", time.Now().Format(time.Stamp), line)
	_, err = logf.Write([]byte(line))
	if err != nil {
		return err
	}
	return nil
}

func launch() error {
	version, err := reg.GetRobloxClientVersion()
	if err != nil {
		log("Error fetching Roblox client version: %s\n", err)
		return err
	}
	rbxKey, err := registry.OpenKey(registry.CLASSES_ROOT, "roblox-player\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		log("Error accessing URI protocol: %s\n", err)
		return err
	}
	defer rbxKey.Close()
	err = rbxKey.SetStringValue("version", version)
	if err != nil {
		log("Error updating URI protocol version: %s\n", err)
		return err
	}
	appDir, err := app.Directory()
	if err != nil {
		log("Error getting app directory: %s\n", err)
		return err
	}
	rbxDir := filepath.Join(appDir, "Versions", version)
	rbxExec := filepath.Join(rbxDir, "RobloxPlayerBeta.exe")
	rbxArgs := []string{}
	if len(os.Args) > 1 {
		rbxArgs = os.Args[1:]
		log("Launch options: %s\n", rbxArgs[0])
	}
	cmd := exec.Command(rbxExec, rbxArgs...)
	cmd.Dir = rbxDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	err = cmd.Start()
	if err != nil {
		log("Error opening Roblox: %s\n", err)
		return err
	}
	namePointer, err := syscall.UTF16PtrFromString("ROBLOX_singletonMutex")
	if err != nil {
		log("Error converting string to pointer: %s\n", err)
		return err
	}
	handle, err := windows.OpenMutex(windows.SYNCHRONIZE, true, namePointer)
	if err == nil {
		syscall.CloseHandle(syscall.Handle(handle))
		return nil
	}
	handle, err = windows.CreateMutex(&windows.SecurityAttributes{}, true, namePointer)
	if err != nil {
		log("Error creating mutex: %s\n", err)
		return err
	}
	defer syscall.CloseHandle(syscall.Handle(handle))
	latestVersion, err := rbxapi.ClientVersionUpload(rbxapi.WindowsBinaryType, rbxapi.LiveChannel)
	if err == nil && latestVersion != version {
		notification := toast.Notification{
			AppID:   "Multiblox",
			Title:   "Your Roblox Client is outdated.",
			Message: fmt.Sprintf("Installed version is %s while the latest version is %s. Enter \"multiblox update\" in command prompt to update.", version, latestVersion),
			Icon:    rbxExec,
		}
		err = notification.Push()
		if err != nil {
			log("Could not display notification: %s", err)
		}
	}
	for {
		exists := false
		processes, err := process.Processes()
		if err != nil {
			return err
		}
		for _, proc := range processes {
			name, err := proc.Name()
			if err != nil {
				continue
			}
			if name == "RobloxPlayerBeta.exe" {
				exists = true
				break
			}
		}
		if !exists {
			break
		}
		time.Sleep(time.Second)
	}
	return nil
}

func main() {
	err := launch()
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}
