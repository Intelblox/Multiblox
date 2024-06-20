package main

import (
	"archive/zip"
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Intelblox/Multiblox/internal/app"
	"github.com/Intelblox/Multiblox/internal/procutil"
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
		return fmt.Errorf("failed to get package manifest: %s", err)
	}
	appDir, err := app.Directory()
	if err != nil {
		return fmt.Errorf("failed to get app directory: %s", err)
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
			err := DownloadPackage(version, pkg, downloadPath)
			if err != nil {
				fmt.Printf("Failed to download package: %s", err)
			}
		}()
		if concurrent >= 3 {
			wg.Wait()
		}
	}
	wg.Wait()
	_, err = os.Stat(installDir)
	if err == nil {
		err = procutil.ReleaseFileHandles(installDir)
		if err != nil {
			return fmt.Errorf("failed to release file handles: %s", err)
		}
		err = os.RemoveAll(installDir)
		if err != nil {
			return fmt.Errorf("failed to remove installation directory: %s", err)
		}
		fmt.Printf("Removed existing installation directory.\n")
	}
	for _, pkg := range pkgs {
		wg.Add(1)
		go func(pkg *Package) {
			defer wg.Done()
			downloadPath := filepath.Join(downloadDir, pkg.Signature)
			installPath := filepath.Join(installDir, pkg.Name)
			err := InstallPackage(pkg, downloadPath, installPath)
			if err != nil {
				fmt.Printf("Failed to install package: %s", err)
			}
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
			return fmt.Errorf("failed to install Microosft Edge Webview: %s", err)
		}
		fmt.Printf("Installed Microsoft Edge Webview.\n")
	}
	err = os.Remove(wvrSetupExec)
	if err != nil {
		return fmt.Errorf("failed to remove Microsoft Edge Webview setup file: %s", err)
	}
	fmt.Printf("Removed setup file for Microsoft Edge Webview.\n")
	appSettingsOriginPath := filepath.Join(appDir, "Roblox", "AppSettings.xml")
	appSettingsOrigin, err := os.Open(appSettingsOriginPath)
	if err != nil {
		return fmt.Errorf("failed to open AppSettings file: %s", err)
	}
	defer appSettingsOrigin.Close()
	appSettingsDestPath := filepath.Join(installDir, "AppSettings.xml")
	appSettingsDest, err := os.Create(appSettingsDestPath)
	if err != nil {
		return fmt.Errorf("failed to create copy of AppSettings file: %s", err)
	}
	defer appSettingsDest.Close()
	_, err = io.Copy(appSettingsDest, appSettingsOrigin)
	if err != nil {
		return fmt.Errorf("failed to copy AppSettings file data: %s", err)
	}
	fmt.Printf("Copied AppSetings.xml into installation directory.\n")
	err = os.Remove(filepath.Join(installDir, "RobloxPlayerLauncher.exe"))
	if err != nil {
		return fmt.Errorf("failed to remove RobloxPlayerLauncher: %s", err)
	}
	fmt.Printf("Removed RobloxPlayerLauncher from the installation directory.\n")
	appKey, err := registry.OpenKey(registry.CURRENT_USER, app.ConfigKey, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to open app key: %s", err)
	}
	err = appKey.SetStringValue("RobloxClientVersion", version)
	appKey.Close()
	if err != nil {
		return fmt.Errorf("failed to update RobloxClientVersion: %s", err)
	}
	fmt.Printf("Updated app key.\n")
	estimatedSize, err := app.EstimatedSize()
	if err != nil {
		return fmt.Errorf("failed to get app estimated size: %s", err)
	}
	uninstallKey, err := registry.OpenKey(registry.CURRENT_USER, app.UninstallKey, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to open uninstall key: %s", err)
	}
	err = uninstallKey.SetDWordValue("EstimatedSize", estimatedSize)
	uninstallKey.Close()
	if err != nil {
		return fmt.Errorf("failed to set estimated size value: %s", err)
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
			return fmt.Errorf("failed to open download file: %s", err)
		}
		hash := md5.New()
		_, err = io.Copy(hash, f)
		f.Close()
		if err != nil {
			return fmt.Errorf("failed to hash file data: %s", err)
		}
		checksum := hex.EncodeToString(hash.Sum(nil))
		if checksum == pkg.Signature {
			return nil
		}
		fmt.Printf("%s: File checksum %s does not match signature %s. Download will continue.\n", pkg.Name, checksum, pkg.Signature)
		err = os.Remove(downloadPath)
		if err != nil {
			return fmt.Errorf("failed to remove download file: %s", err)
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
			return fmt.Errorf("%s: failed to create directory at %s: %s", pkg.Name, downloadDir, err)
		}
		var f *os.File
		f, err = os.Create(downloadPath)
		if err != nil {
			return fmt.Errorf("%s: failed to create file at %s: %s", pkg.Name, downloadPath, err)
		}
		defer f.Close()
		_, err = io.Copy(f, resp.Body)
		if err != nil {
			return fmt.Errorf("%s: failed to write data to file: %s", pkg.Name, err)
		}
		f.Close()
		f, err := os.Open(downloadPath)
		if err != nil {
			return fmt.Errorf("%s: failed to open file: %s", pkg.Name, err)
		}
		hash := md5.New()
		_, err = io.Copy(hash, f)
		f.Close()
		if err != nil {
			return fmt.Errorf("%s: failed to hash file data: %s", pkg.Name, err)
		}
		checksum := hex.EncodeToString(hash.Sum(nil))
		if checksum != pkg.Signature {
			return fmt.Errorf("%s: checksum %s does not match signature %s", pkg.Name, checksum, pkg.Signature)
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
			return fmt.Errorf("%s: failed to create directory %s: %s", pkg.Name, installDir, err)
		}
		og, err := os.Open(downloadPath)
		if err != nil {
			return fmt.Errorf("%s: failed to open file %s: %s", pkg.Name, downloadPath, err)
		}
		defer og.Close()
		cp, err := os.Create(installPath)
		if err != nil {
			return fmt.Errorf("%s: failed to create file at %s: %s", pkg.Name, installPath, err)
		}
		defer cp.Close()
		_, err = io.Copy(cp, og)
		if err != nil {
			return fmt.Errorf("%s: failed to copy file data: %s", pkg.Name, err)
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
			return fmt.Errorf("%s: failed to open zip file: %s", pkg.Name, err)
		}
		for _, file := range zipr.File {
			if file.Mode().IsDir() {
				continue
			}
			installPath := filepath.Join(installPath, file.Name)
			installDir := filepath.Dir(installPath)
			err := os.MkdirAll(installDir, os.ModeDir)
			if err != nil {
				return fmt.Errorf("%s: failed to create directory at %s: %s", pkg.Name, installDir, err)
			}
			og, err := file.Open()
			if err != nil {
				return fmt.Errorf("%s: failed to open compressed file at %s: %s", pkg.Name, file.Name, err)
			}
			defer og.Close()
			cp, err := os.Create(installPath)
			if err != nil {
				return fmt.Errorf("%s: failed to create file at %s: %s", pkg.Name, file.Name, err)
			}
			defer cp.Close()
			_, err = io.Copy(cp, og)
			if err != nil {
				return fmt.Errorf("%s: failed to copy file data: %s", pkg.Name, err)
			}
		}
		fmt.Printf("%s: Extracted to installation directory.\n", pkg.Name)
	}
	return nil
}
