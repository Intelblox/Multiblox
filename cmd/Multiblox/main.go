package main

import (
	_ "embed"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"os"
	"path/filepath"

	"app"
	"rblxapi"
	"regconf"

	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func help() error {
	fmt.Printf("USAGE: multiblox [help|version|update|reinstall|uninstall|logs]\n")
	return nil
}

func version() error {
	fmt.Printf("Multiblox v%s\n", app.Version)
	return nil
}

func installRobloxClient(version string) error {
	pkgs, err := rblxapi.PackageManifest(version)
	if err != nil {
		return err
	}
	appDir, err := app.Directory()
	if err != nil {
		return err
	}
	downloadDir := filepath.Join(appDir, "Downloads")
	installDir := filepath.Join(appDir, "Versions", version)
	var wg sync.WaitGroup
	concurrent := 0
	for _, pkg := range pkgs {
		wg.Add(1)
		concurrent += 1
		go func() {
			defer wg.Done()
			downloadPath := filepath.Join(downloadDir, pkg.Signature)
			rblxapi.DownloadPackage(version, pkg, downloadPath)
		}()
		if concurrent >= 3 {
			wg.Wait()
		}
	}
	wg.Wait()
	processes, err := process.Processes()
	if err != nil {
		return err
	}
	_, err = os.Stat(installDir)
	if err == nil {
		for _, proc := range processes {
			execPath, err := proc.Exe()
			if err != nil {
				continue
			}
			if !strings.HasPrefix(execPath, installDir) {
				continue
			}
			fmt.Printf("Killing process occupying installation directory.\n")
			err = proc.Kill()
			if err != nil {
				return err
			}
			time.Sleep(time.Second)
		}
		fmt.Printf("Removing existing installation directory.\n")
		err = os.RemoveAll(installDir)
		if err != nil {
			return err
		}
	}
	for _, pkg := range pkgs {
		wg.Add(1)
		go func(pkg *rblxapi.Package) {
			defer wg.Done()
			downloadPath := filepath.Join(downloadDir, pkg.Signature)
			installPath := filepath.Join(installDir, pkg.Name)
			rblxapi.InstallPackage(pkg, downloadPath, installPath)
		}(pkg)
	}
	wg.Wait()
	webviewRuntimeInstalled := false
	webviewRuntimeK, err := registry.OpenKey(registry.LOCAL_MACHINE, "SOFTWARE\\WOW6432Node\\Microsoft\\EdgeUpdate\\Clients\\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}", registry.READ)
	if err == nil {
		webviewRuntimeInstalled = true
	}
	webviewRuntimeK.Close()
	webviewRuntimeK, err = registry.OpenKey(registry.CURRENT_USER, "Software\\Microsoft\\EdgeUpdate\\Clients\\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}", registry.READ)
	if err == nil {
		webviewRuntimeInstalled = true
	}
	webviewRuntimeK.Close()
	webviewSetupExec := filepath.Join(installDir, "MicrosoftEdgeWebview2Setup.exe")
	if !webviewRuntimeInstalled {
		fmt.Printf("Installing Microsoft Edge Webview.\n")
		webviewSetupCmd := exec.Command(webviewSetupExec, "/silent", "/install")
		err = webviewSetupCmd.Run()
		if err != nil {
			return err
		}
	}
	err = os.Remove(webviewSetupExec)
	if err != nil {
		return err
	}
	fmt.Printf("Writing AppSettings.xml into installlation directory.\n")
	asOriginPath := filepath.Join(appDir, "Roblox", "AppSettings.xml")
	asOrigin, err := os.Open(asOriginPath)
	if err != nil {
		return err
	}
	defer asOrigin.Close()
	asCopyPath := filepath.Join(installDir, "AppSettings.xml")
	asCopy, err := os.Create(asCopyPath)
	if err != nil {
		return err
	}
	defer asCopy.Close()
	_, err = io.Copy(asCopy, asOrigin)
	if err != nil {
		return err
	}
	fmt.Printf("Disabling Roblox auto-update feature.\n")
	err = os.Remove(filepath.Join(installDir, "RobloxPlayerLauncher.exe"))
	if err != nil {
		return err
	}
	rblxk, err := registry.OpenKey(registry.CLASSES_ROOT, "roblox-player\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	err = rblxk.SetStringValue("version", version)
	if err != nil {
		return err
	}
	rblxk.Close()
	rblxk, err = registry.OpenKey(registry.CLASSES_ROOT, "roblox\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	err = rblxk.SetStringValue("version", version)
	rblxk.Close()
	if err != nil {
		return err
	}
	regconf.SetRobloxClientVersion(version)
	estimatedSize, err := app.EstimatedSize()
	if err != nil {
		return err
	}
	uninstallk, _, err := registry.CreateKey(registry.CURRENT_USER, app.UninstallKey, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	err = uninstallk.SetDWordValue("EstimatedSize", estimatedSize)
	if err != nil {
		return err
	}
	return nil
}

func update() error {
	rcv, err := regconf.GetRobloxClientVersion()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if rcv != "" {
		fmt.Printf("Current ROBLOX client version: %s\n", rcv)
	}
	cvu, err := rblxapi.ClientVersionUpload(rblxapi.WindowsBinaryType, rblxapi.LiveChannel)
	if err != nil {
		return err
	}
	if rcv == cvu {
		fmt.Printf("You are currently up to date.")
		return nil
	} else {
		fmt.Printf("Latest ROBLOX client version: %s\n", cvu)
	}
	installRobloxClient(cvu)
	return nil
}

func reinstall() error {
	cvu, err := rblxapi.ClientVersionUpload(rblxapi.WindowsBinaryType, rblxapi.LiveChannel)
	if err != nil {
		return err
	}
	installRobloxClient(cvu)
	return nil
}

func uninstall() error {
	appDir, err := app.Directory()
	if err != nil {
		return err
	}
	fmt.Printf("App directory: %s\n", appDir)
	envk, err := registry.OpenKey(registry.CURRENT_USER, "Environment", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer envk.Close()
	path, _, err := envk.GetStringValue("Path")
	if err != nil {
		return err
	}
	if strings.Contains(path, appDir) {
		fmt.Println("Removing app directory from PATH.")
		newDirs := []string{}
		dirs := strings.Split(path, ";")
		for _, dir := range dirs {
			if dir != appDir {
				newDirs = append(newDirs, dir)
			}
		}
		path = strings.Join(newDirs, ";")
		envk.SetStringValue("Path", path)
	}
	fmt.Println("Restoring Roblox URI protocol to default.")
	rblxk, err := registry.OpenKey(registry.CLASSES_ROOT, "roblox-player\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer rblxk.Close()
	version, _, err := rblxk.GetStringValue("version")
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cmdString := filepath.Join(home, "AppData", "Local", "Roblox", "Versions", version, "RobloxPlayerBeta.exe")
	err = rblxk.SetStringValue("", cmdString)
	if err != nil {
		return err
	}
	rblxk, err = registry.OpenKey(registry.CLASSES_ROOT, "roblox\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	err = rblxk.SetStringValue("", cmdString)
	rblxk.Close()
	if err != nil {
		return err
	}
	fmt.Printf("Uninstall key: %s\n", app.UninstallKey)
	fmt.Println("Removing uninstall key.")
	err = registry.DeleteKey(registry.CURRENT_USER, app.UninstallKey)
	if err != nil {
		return err
	}
	fmt.Println("Writing uninstall.bat into temp directory.")
	uninstallOriginPath := filepath.Join(appDir, "uninstall.bat")
	uninstallOrigin, err := os.Open(uninstallOriginPath)
	if err != nil {
		return err
	}
	defer uninstallOrigin.Close()
	uninstallPath := filepath.Join(os.TempDir(), "multiblox-uninstall.bat")
	uninstallF, err := os.Create(uninstallPath)
	if err != nil {
		return err
	}
	defer uninstallF.Close()
	_, err = io.Copy(uninstallF, uninstallOrigin)
	if err != nil {
		return err
	}
	uninstallOrigin.Close()
	uninstallF.Close()
	processes, err := process.Processes()
	if err != nil {
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
	}
	fmt.Println("Running uninstall.bat as a detached process.")
	cmd := exec.Command(uninstallPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	fmt.Println("Exiting.")
	return nil
}

func logs() error {
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
	cmd := help
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "help":
			cmd = help
		case "version":
			cmd = version
		case "update":
			cmd = update
		case "reinstall":
			cmd = reinstall
		case "uninstall":
			cmd = uninstall
		case "logs":
			cmd = logs
		}
	}
	err := cmd()
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
	}
}
