package rblxapi

import (
	"archive/zip"
	"bufio"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var ClientSettingsEndpoints = []string{"https://clientsettingscdn.roblox.com", "https://clientsettings.roblox.com"}

type ClientVersionResult struct {
	Version             string `json:"version"`
	ClientVersionUpload string `json:"clientVersionUpload"`
	BootstrapperVersion string `json:"bootstrapperVersion"`
}

const (
	WindowsBinaryType = "WindowsPlayer"
)

const (
	LiveChannel = "LIVE"
)

func ClientVersion(binaryType string, channel string) (*ClientVersionResult, error) {
	path := fmt.Sprintf("/v2/client-version/%s/channel/%s", binaryType, channel)
	var resp *http.Response
	var err error
	for _, endpoint := range ClientSettingsEndpoints {
		url := endpoint + path
		resp, err = http.Get(url)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	var clientVersion *ClientVersionResult
	err = json.NewDecoder(resp.Body).Decode(&clientVersion)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return clientVersion, nil
}

func ClientVersionUpload(binaryType string, channel string) (string, error) {
	clientVersion, err := ClientVersion(binaryType, channel)
	if err != nil {
		return "", err
	}
	clientVersionUpload := clientVersion.ClientVersionUpload
	return clientVersionUpload, nil
}

type Package struct {
	Name          string `json:"name"`
	Signature     string `json:"signature"`
	RawPackedSize string `json:"rawPackedSize"`
	RawSize       string `json:"rawSize"`
}

var Endpoints = []string{"https://setup.rbxcdn.com", "https://setup-ak.rbxcdn.com", "https://roblox-setup.cachefly.net", "https://s3.amazonaws.com/setup.roblox.com"}

func PackageManifest(version string) ([]*Package, error) {
	fmt.Printf("Fetching package manifest for %s.\n", version)
	path := fmt.Sprintf("/%s-rbxPkgManifest.txt", version)
	resp, err := http.Get(Endpoints[0] + path)
	if err != nil {
		fmt.Printf("Package manifest: Error downloading manifest: %s", err)
		return nil, err
	}
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
		fmt.Printf("%s: Writing data to installation directory.\n", pkg.Name)
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
			fmt.Printf("%s: Error writing data from origin to copy: %s\n", pkg.Name, err)
			return err
		}
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
		fmt.Printf("%s: Extracting zip file into installation directory.\n", pkg.Name)
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
				fmt.Printf("%s: Trying to create file %s", pkg.Name, installPath)
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
	}
	return nil
}
