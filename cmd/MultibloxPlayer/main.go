package main

import (
	"app"
	_ "embed"
	"fmt"
	"rblxapi"
	"regconf"

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
	log("Applying registry settings.")
	version, err := regconf.GetRobloxClientVersion()
	if err != nil {
		return err
	}
	rblxKey, err := registry.OpenKey(registry.CLASSES_ROOT, "roblox-player\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer rblxKey.Close()
	err = rblxKey.SetStringValue("version", version)
	if err != nil {
		return err
	}
	rblxKey, err = registry.OpenKey(registry.CLASSES_ROOT, "roblox\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer rblxKey.Close()
	err = rblxKey.SetStringValue("version", version)
	if err != nil {
		return err
	}
	appDir, err := app.Directory()
	if err != nil {
		return err
	}
	rblxDir := filepath.Join(appDir, "Versions", version)
	rblxExec := filepath.Join(rblxDir, "RobloxPlayerBeta.exe")
	rblxArgs := []string{}
	if len(os.Args) > 1 {
		rblxArgs = os.Args[1:]
		log("Launch options: %s", rblxArgs[0])
	}
	cmd := exec.Command(rblxExec, rblxArgs...)
	cmd.Dir = rblxDir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	namePointer, err := syscall.UTF16PtrFromString("ROBLOX_singletonMutex")
	if err != nil {
		return err
	}
	handle, err := windows.OpenMutex(windows.SYNCHRONIZE, true, namePointer)
	if err == nil {
		syscall.CloseHandle(syscall.Handle(handle))
		return nil
	}
	handle, err = windows.CreateMutex(&windows.SecurityAttributes{}, true, namePointer)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(syscall.Handle(handle))
	latestVersion, err := rblxapi.ClientVersionUpload(rblxapi.WindowsBinaryType, rblxapi.LiveChannel)
	if err == nil && latestVersion != version {
		notification := toast.Notification{
			AppID:   "Multiblox",
			Title:   "Your Roblox Client is outdated.",
			Message: fmt.Sprintf("Installed version is %s while the latest version is %s. Enter \"multiblox update\" in command prompt to update.", version, latestVersion),
			Icon:    rblxExec,
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
