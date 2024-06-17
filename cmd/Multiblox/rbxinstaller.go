package main

import (
	"archive/zip"
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Intelblox/Multiblox/internal/app"
	"github.com/shirou/gopsutil/v4/process"
	"golang.org/x/sys/windows/registry"
)

type Package struct {
	Name          string `json:"name"`
	Signature     string `json:"signature"`
	RawPackedSize string `json:"rawPackedSize"`
	RawSize       string `json:"rawSize"`
}

var Endpoints = []string{"https://setup.rbxcdn.com", "https://setup-ak.rbxcdn.com", "https://roblox-setup.cachefly.net", "https://s3.amazonaws.com/setup.roblox.com"}

func InstallRobloxClient(version string) error {
	pkgs, err := PackageManifest(version)
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
			DownloadPackage(version, pkg, downloadPath)
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
			err = proc.Kill()
			if err != nil {
				fmt.Printf("Error killing process occupying installation directory: %s\n", err)
				return err
			}
			fmt.Printf("Killed process occupying installation directory.\n")
			time.Sleep(time.Second)
		}
		err = os.RemoveAll(installDir)
		if err != nil {
			fmt.Printf("Could not remove existing installation directory: %s\n", err)
			return err
		}
		fmt.Printf("Removed existing installation directory.\n")
	}
	for _, pkg := range pkgs {
		wg.Add(1)
		go func(pkg *Package) {
			defer wg.Done()
			downloadPath := filepath.Join(downloadDir, pkg.Signature)
			installPath := filepath.Join(installDir, pkg.Name)
			InstallPackage(pkg, downloadPath, installPath)
		}(pkg)
	}
	wg.Wait()
	wvrInstalled := false
	wvrKey, err := registry.OpenKey(registry.LOCAL_MACHINE, "SOFTWARE\\WOW6432Node\\Microsoft\\EdgeUpdate\\Clients\\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}", registry.READ)
	if err == nil {
		wvrInstalled = true
	}
	wvrKey.Close()
	wvrKey, err = registry.OpenKey(registry.CURRENT_USER, "Software\\Microsoft\\EdgeUpdate\\Clients\\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}", registry.READ)
	if err == nil {
		wvrInstalled = true
	}
	wvrKey.Close()
	wvrSetupExec := filepath.Join(installDir, "MicrosoftEdgeWebview2Setup.exe")
	if !wvrInstalled {
		fmt.Printf("Microsoft Edge Webview not installed.\n")
		webviewSetupCmd := exec.Command(wvrSetupExec, "/silent", "/install")
		err = webviewSetupCmd.Run()
		if err != nil {
			fmt.Printf("Error installing Microsoft Edge Webview: %s\n", err)
			return err
		}
		fmt.Printf("Installed Microsoft Edge Webview.\n")
	}
	err = os.Remove(wvrSetupExec)
	if err != nil {
		return err
	}
	fmt.Printf("Removed setup file for Microsoft Edge Webview.\n")
	appSettingsOriginPath := filepath.Join(appDir, "Roblox", "AppSettings.xml")
	appSettingsOrigin, err := os.Open(appSettingsOriginPath)
	if err != nil {
		fmt.Printf("Error opening AppSettings in the assets directory: %s\n", err)
		return err
	}
	defer appSettingsOrigin.Close()
	appSettingsDestPath := filepath.Join(installDir, "AppSettings.xml")
	appSettingsDest, err := os.Create(appSettingsDestPath)
	if err != nil {
		fmt.Printf("Error creating AppSettings for the client: %s\n", err)
		return err
	}
	defer appSettingsDest.Close()
	_, err = io.Copy(appSettingsDest, appSettingsOrigin)
	if err != nil {
		fmt.Printf("Error copying AppSettings into installation directory: %s\n", err)
		return err
	}
	fmt.Printf("Copied AppSetings.xml into installation directory.\n")
	err = os.Remove(filepath.Join(installDir, "RobloxPlayerLauncher.exe"))
	if err != nil {
		fmt.Printf("Error removing RobloxPlayerLauncher from the installation directory: %s\n", err)
		return err
	}
	fmt.Printf("Removed RobloxPlayerLauncher from the installation directory.\n")
	rbxKey, err := registry.OpenKey(registry.CLASSES_ROOT, "roblox-player\\shell\\open\\command", registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("Error opening Roblox URI protocol key: %s\n", err)
		return err
	}
	err = rbxKey.SetStringValue("version", version)
	rbxKey.Close()
	if err != nil {
		fmt.Printf("Error updating Roblox registry key: %s\n", err)
		return err
	}
	fmt.Printf("Updated Roblox registry key.\n")
	appKey, _, err := registry.CreateKey(registry.CURRENT_USER, app.ConfigKey, registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("Error accessing Multiblox registry key: %s\n", err)
		return err
	}
	err = appKey.SetStringValue("RobloxClientVersion", version)
	appKey.Close()
	if err != nil {
		fmt.Printf("Error updating Multiblox registry key: %s\n", err)
		return err
	}
	fmt.Printf("Updated Multiblox registry key.\n")
	estimatedSize, err := app.EstimatedSize()
	if err != nil {
		fmt.Printf("Error calculating estimated size: %s\n", err)
		return err
	}
	uninstallKey, _, err := registry.CreateKey(registry.CURRENT_USER, app.UninstallKey, registry.ALL_ACCESS)
	if err != nil {
		fmt.Printf("Error accessing uninstall key: %s\n", err)
		return err
	}
	err = uninstallKey.SetDWordValue("EstimatedSize", estimatedSize)
	if err != nil {
		fmt.Printf("Error updating uninstall key: %s\n", err)
		return err
	}
	fmt.Printf("Updated uninstall key.\n")
	return nil
}

func PackageManifest(version string) ([]*Package, error) {
	path := fmt.Sprintf("/%s-rbxPkgManifest.txt", version)
	resp, err := http.Get(Endpoints[0] + path)
	if err != nil {
		fmt.Printf("Package manifest: Error downloading manifest: %s", err)
		return nil, err
	}
	fmt.Printf("Downloaded package manifest for %s.\n", version)
	pkgs := []*Package{}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Scan()
	for scanner.Scan() {
		pkg := &Package{}
		pkg.Name = scanner.Text()
		scanner.Scan()
		pkg.Signature = scanner.Text()
		scanner.Scan()
		pkg.RawPackedSize = scanner.Text()
		scanner.Scan()
		pkg.RawSize = scanner.Text()
		pkgs = append(pkgs, pkg)
	}
	resp.Body.Close()
	return pkgs, nil
}

func DownloadPackage(version string, pkg *Package, downloadPath string) error {
	fmt.Printf("%s: Preparing download.\n", pkg.Name)
	_, err := os.Stat(downloadPath)
	if err == nil {
		fmt.Printf("%s: Package already exists.\n", pkg.Name)
		f, err := os.Open(downloadPath)
		if err != nil {
			return err
		}
		hash := md5.New()
		_, err = io.Copy(hash, f)
		f.Close()
		if err != nil {
			return err
		}
		checksum := hex.EncodeToString(hash.Sum(nil))
		if checksum == pkg.Signature {
			return nil
		}
		fmt.Printf("%s: File checksum %s does not match signature %s. Download will continue.\n", pkg.Name, checksum, pkg.Signature)
		err = os.Remove(downloadPath)
		if err != nil {
			return err
		}
	}
	path := fmt.Sprintf("/%s-%s", version, pkg.Name)
	url := Endpoints[0] + path
	attemptLimit := 3
	for attempt := 1; attempt <= attemptLimit; attempt++ {
		if attempt == 1 {
			fmt.Printf("%s: Downloading.\n", pkg.Name)
		} else {
			fmt.Printf("%s: Retrying download for attempt %d/%d.\n", pkg.Name, attempt, attemptLimit)
		}
		var resp *http.Response
		resp, err = http.Get(url)
		if err != nil {
			fmt.Printf("%s: Error from %s: %s\n", pkg.Name, url, err)
			continue
		}
		defer resp.Body.Close()
		downloadDir := filepath.Dir(downloadPath)
		err = os.MkdirAll(downloadDir, os.ModeDir)
		if err != nil {
			fmt.Printf("%s: Error creating directory at %s: %s\n", pkg.Name, downloadDir, err)
			return err
		}
		var f *os.File
		f, err = os.Create(downloadPath)
		if err != nil {
			fmt.Printf("%s: Error creating file at %s: %s\n", pkg.Name, downloadPath, err)
			return err
		}
		defer f.Close()
		_, err = io.Copy(f, resp.Body)
		if err != nil {
			fmt.Printf("%s: Error writing data to file: %s\n", pkg.Name, err)
			return err
		}
		f.Close()
		f, err := os.Open(downloadPath)
		if err != nil {
			fmt.Printf("%s: Error opening file: %s \n", pkg.Name, err)
			return err
		}
		hash := md5.New()
		_, err = io.Copy(hash, f)
		f.Close()
		if err != nil {
			fmt.Printf("%s: Error copying data to hash: %s\n", pkg.Name, err)
			return err
		}
		checksum := hex.EncodeToString(hash.Sum(nil))
		if checksum != pkg.Signature {
			e := fmt.Sprintf("%s: File checksum %s does not match signature %s.", pkg.Name, checksum, pkg.Signature)
			fmt.Println(e)
			return errors.New(e)
		}
		fmt.Printf("%s: Download completed.\n", pkg.Name)
		break
	}
	return err
}

func InstallPackage(pkg *Package, downloadPath string, installPath string) error {
	if strings.HasSuffix(pkg.Name, ".exe") {
		installDir := filepath.Dir(installPath)
		err := os.MkdirAll(installDir, os.ModeDir)
		if err != nil {
			fmt.Printf("%s: Could not create directory %s: %s\n", pkg.Name, installDir, err)
			return err
		}
		og, err := os.Open(downloadPath)
		if err != nil {
			fmt.Printf("%s: Error opening %s: %s\n", pkg.Name, downloadPath, err)
			return err
		}
		defer og.Close()
		cp, err := os.Create(installPath)
		if err != nil {
			fmt.Printf("%s: Error creating file at %s: %s\n", pkg.Name, installPath, err)
			return err
		}
		defer cp.Close()
		_, err = io.Copy(cp, og)
		if err != nil {
			fmt.Printf("%s: Error writing fto installation directory: %s\n", pkg.Name, err)
			return err
		}
		fmt.Printf("%s: Wrote to installation directory.\n", pkg.Name)
	} else if strings.HasSuffix(pkg.Name, ".zip") {
		distribution := map[string]string{
			"shaders.zip":                   "shaders",
			"ssl.zip":                       "ssl",
			"content-avatar.zip":            "content\\avatar",
			"content-configs.zip":           "content\\configs",
			"content-fonts.zip":             "content\\fonts",
			"content-sky.zip":               "content\\sky",
			"content-sounds.zip":            "content\\sounds",
			"content-textures2.zip":         "content\\textures",
			"content-models.zip":            "content\\models",
			"content-textures3.zip":         "PlatformContent\\pc\\textures",
			"content-terrain.zip":           "PlatformContent\\pc\\terrain",
			"content-platform-fonts.zip":    "PlatformContent\\pc\\fonts",
			"extracontent-luapackages.zip":  "ExtraContent\\LuaPackages",
			"extracontent-translations.zip": "ExtraContent\\translations",
			"extracontent-models.zip":       "ExtraContent\\models",
			"extracontent-textures.zip":     "ExtraContent\\textures",
			"extracontent-places.zip":       "ExtraContent\\places",
		}
		installPath = filepath.Dir(installPath)
		subDir, exists := distribution[pkg.Name]
		if exists {
			installPath = filepath.Join(installPath, subDir)
		}
		zipr, err := zip.OpenReader(downloadPath)
		if err != nil {
			fmt.Printf("%s: Error opening zip file: %s\n", pkg.Name, err)
			return err
		}
		for _, file := range zipr.File {
			if file.Mode().IsDir() {
				continue
			}
			installPath := filepath.Join(installPath, file.Name)
			installDir := filepath.Dir(installPath)
			err := os.MkdirAll(installDir, os.ModeDir)
			if err != nil {
				fmt.Printf("%s: Error creating directory at %s: %s\n", pkg.Name, installDir, err)
				return err
			}
			og, err := file.Open()
			if err != nil {
				fmt.Printf("%s: Error opening compressed file at %s: %s\n", pkg.Name, file.Name, err)
				return err
			}
			defer og.Close()
			cp, err := os.Create(installPath)
			if err != nil {
				fmt.Printf("%s: Error creating file at %s: %s\n", pkg.Name, installPath, err)
				return err
			}
			defer cp.Close()
			_, err = io.Copy(cp, og)
			if err != nil {
				fmt.Printf("%s: Error uncompressing data of %s: %s\n", pkg.Name, file.Name, err)
				return err
			}
		}
		fmt.Printf("%s: Extracted to installation directory.\n", pkg.Name)
	}
	return nil
}
