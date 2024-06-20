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
		return fmt.Errorf("failed to get app directory: %s", err)
	}
	dir := filepath.Join(appDir, "Logs")
	path := filepath.Join(dir, "player.log")
	err = os.MkdirAll(dir, os.ModeDir)
	if err != nil {
		return fmt.Errorf("failed to create logs directory: %s", err)
	}
	logf, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		return fmt.Errorf("failed to open log file: %s", err)
	}
	defer logf.Close()
	line := fmt.Sprintf(format, message...)
	line = fmt.Sprintf("[%s] %s", time.Now().Format(time.Stamp), line)
	_, err = logf.Write([]byte(line))
	if err != nil {
		return fmt.Errorf("failed to write log file: %s", err)
	}
	return nil
}

func Notify(topic string, message string) error {
	appDir, err := app.Directory()
	if err != nil {
		return fmt.Errorf("failed to get app directory: %s", err)
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
		return fmt.Errorf("failed to push notification: %s", err)
	}
	return nil
}

func Launch() error {
	version, err := reg.Get("RobloxClientVersion")
	if err != nil {
		return fmt.Errorf("failed to get current Roblox version: %s", err)
	}
	appDir, err := app.Directory()
	if err != nil {
		return fmt.Errorf("failed to get app directory: %s", err)
	}
	mbxDir := filepath.Join(appDir, "Versions", version)
	mbxRbxExe := filepath.Join(mbxDir, "RobloxPlayerBeta.exe")
	_, err = os.Stat(mbxRbxExe)
	if os.IsNotExist(err) {
		err = Notify("Roblox not installed", "Run \"mbx install roblox\" in command prompt to fix.")
		if err != nil {
			return fmt.Errorf("failed to send notification: %s", err)
		}
	}
	for _, name := range []string{"roblox", "roblox-player"} {
		rbxKey, err := registry.OpenKey(registry.CLASSES_ROOT, fmt.Sprintf("%s\\shell\\open\\command", name), registry.ALL_ACCESS)
		if err != nil {
			return fmt.Errorf("failed to open roblox key: %s", err)
		}
		defer rbxKey.Close()
		err = rbxKey.SetStringValue("version", version)
		if err != nil {
			return fmt.Errorf("failed to update URI protocol version: %s", err)
		}
		rbxKey.Close()
	}
	rbxArgs := []string{}
	if len(os.Args) > 1 {
		rbxArgs = os.Args[1:]
	}
	cmd := exec.Command(mbxRbxExe, rbxArgs...)
	cmd.Dir = mbxDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to open Roblox: %s", err)
	}
	appKey, err := registry.OpenKey(registry.CURRENT_USER, app.ConfigKey, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to open app key: %s", err)
	}
	multiinstancing, _, err := appKey.GetIntegerValue("MultiInstancing")
	if err != nil {
		return fmt.Errorf("failed to get MultiInstancing value: %s", err)
	}
	discordRichPresence, _, err := appKey.GetIntegerValue("DiscordRichPresence")
	if err != nil {
		return fmt.Errorf("failed to get DiscordRichPresence value: %s", err)
	}
	updateNotifications, _, err := appKey.GetIntegerValue("UpdateNotifications")
	if err != nil {
		return fmt.Errorf("failed to get UpdateNotifications value: %s", err)
	}
	updateNotificationFrequency, _, err := appKey.GetIntegerValue("UpdateNotificationFrequency")
	if err != nil {
		return fmt.Errorf("failed to get UpdateNotificationFrequency value: %s", err)
	}
	lastUpdateNotification, _, err := appKey.GetIntegerValue("LastUpdateNotification")
	if err != nil {
		return fmt.Errorf("failed to get LastUpdateNotification value: %s", err)
	}
	currentTime := uint64(time.Now().Unix())
	if updateNotifications == 1 && currentTime-lastUpdateNotification > updateNotificationFrequency {
		latestVersion, err := rbxapi.ClientVersionUpload(rbxapi.WindowsBinaryType, rbxapi.LiveChannel)
		if err == nil && latestVersion != version {
			Notify("Your Roblox Client is outdated.", "Enter \"mbx update\" in command prompt to update.")
			err = appKey.SetQWordValue("LastUpdateNotification", currentTime)
			if err != nil {
				return fmt.Errorf("failed to set LastUpdateNotificatoin value: %s", err)
			}
		}
	}
	var holdingMutex bool
	if multiinstancing == 1 {
		namePointer, err := syscall.UTF16PtrFromString("ROBLOX_singletonMutex")
		if err != nil {
			return fmt.Errorf("failed to convert name to pointer: %s", err)
		}
		robloxSingletonMutex, err := windows.CreateMutex(&windows.SecurityAttributes{}, true, namePointer)
		if err != nil && err != windows.ERROR_ALREADY_EXISTS {
			return fmt.Errorf("failed to create Roblox singleton mutex: %s", err)
		} else if err == nil {
			holdingMutex = true
			defer syscall.CloseHandle(syscall.Handle(robloxSingletonMutex))
		}
	}
	if discordRichPresence == 1 {
		go func() {
			err = DiscordRPC()
			if err != nil {
				err = fmt.Errorf("failed to continue running Discord RPC: %s", err)
			}
		}()
	}
	err = cmd.Wait()
	if err != nil {
		err = fmt.Errorf("failed to wait for Roblox to exit")
	}
	if holdingMutex {
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
	return err
}

func main() {
	err := Launch()
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}
