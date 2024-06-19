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

func Log(format string, message ...any) error {
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

func Notify(topic string, message string) error {
	appDir, err := app.Directory()
	if err != nil {
		Log("Error getting app directory: %s\n", err)
		return err
	}
	appIcon := filepath.Join(appDir, "icon.ico")
	notification := toast.Notification{
		AppID:   "Multiblox",
		Title:   topic,
		Message: message,
		Icon:    appIcon,
	}
	err = notification.Push()
	if err != nil {
		Log("Could not display notification: %s", err)
		return err
	}
	return nil
}

func Launch() error {
	version, err := reg.Get("RobloxClientVersion")
	if err != nil {
		Log("Error fetching Roblox client version: %s\n", err)
		return err
	}
	appDir, err := app.Directory()
	if err != nil {
		Log("Error getting app directory: %s\n", err)
		return err
	}
	rbxDir := filepath.Join(appDir, "Versions", version)
	rbxExec := filepath.Join(rbxDir, "RobloxPlayerBeta.exe")
	_, err = os.Stat(rbxExec)
	if os.IsNotExist(err) {
		err = Notify("Roblox not installed", "Run \"mbx install roblox\" in command prompt to fix.")
		if err != nil {
			return err
		}
	}
	rbxKey, err := registry.OpenKey(registry.CLASSES_ROOT, "roblox-player\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		Log("Error accessing URI protocol: %s\n", err)
		return err
	}
	defer rbxKey.Close()
	err = rbxKey.SetStringValue("version", version)
	if err != nil {
		Log("Error updating URI protocol version: %s\n", err)
		return err
	}
	rbxArgs := []string{}
	if len(os.Args) > 1 {
		rbxArgs = os.Args[1:]
		Log("Launch options: %s\n", rbxArgs[0])
	}
	cmd := exec.Command(rbxExec, rbxArgs...)
	cmd.Dir = rbxDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	err = cmd.Start()
	if err != nil {
		Log("Error opening Roblox: %s\n", err)
		return err
	}
	appKey, err := registry.OpenKey(registry.CURRENT_USER, app.ConfigKey, registry.ALL_ACCESS)
	if err != nil {
		Log("Error opening application key: %s\n", err)
		return err
	}
	multiinstancing, _, err := appKey.GetIntegerValue("MultiInstancing")
	if err != nil {
		Log("Error getting MultiInstancing value: %s\n", err)
		return err
	}
	updateNotifications, _, err := appKey.GetIntegerValue("UpdateNotifications")
	if err != nil {
		Log("Error getting UpdateNotifications value: %s\n", err)
		return err
	}
	updateNotificationFrequency, _, err := appKey.GetIntegerValue("UpdateNotificationFrequency")
	if err != nil {
		Log("Error getting UpdateNotificationFrequency value:%s\n", err)
		return err
	}
	lastUpdateNotification, _, err := appKey.GetIntegerValue("LastUpdateNotification")
	if err != nil {
		Log("Error getting LastUpdateNotification value: %s\n", err)
		return err
	}
	currentTime := uint64(time.Now().Unix())
	if updateNotifications == 1 && currentTime-lastUpdateNotification > updateNotificationFrequency {
		latestVersion, err := rbxapi.ClientVersionUpload(rbxapi.WindowsBinaryType, rbxapi.LiveChannel)
		if err == nil && latestVersion != version {
			Notify("Your Roblox Client is outdated.", "Enter \"mbx update\" in command prompt to update.")
			err = appKey.SetQWordValue("LastUpdateNotification", currentTime)
			if err != nil {
				Log("Error setting LastUpdateNotification value: %s\n", err)
				return err
			}
		}
	}
	if multiinstancing == 1 {
		namePointer, err := syscall.UTF16PtrFromString("ROBLOX_singletonMutex")
		if err != nil {
			Log("Error converting string to pointer: %s\n", err)
			return err
		}
		handle, err := windows.OpenMutex(windows.SYNCHRONIZE, true, namePointer)
		if err == nil {
			syscall.CloseHandle(syscall.Handle(handle))
			return nil
		}
		handle, err = windows.CreateMutex(&windows.SecurityAttributes{}, true, namePointer)
		if err != nil {
			Log("Error creating mutex: %s\n", err)
			return err
		}
		defer syscall.CloseHandle(syscall.Handle(handle))
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
	}
	return nil
}

func main() {
	err := Launch()
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}
