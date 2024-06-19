package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

type GithubAuthor struct {
	Login             string `json:"login"`
	Id                int    `json:"id"`
	NodeId            string `json:"node_id"`
	AvatarUrl         string `json:"avatar_url"`
	GravatarId        string `json:"gravatar_id"`
	Url               string `json:"url"`
	HtmlUrl           string `json:"html_url"`
	FollowersUrl      string `json:"followers_url"`
	FollowingUrl      string `json:"following_url"`
	GistsUrl          string `json:"gists_url"`
	StarredUrl        string `json:"starred_url"`
	SubscriptionsUrl  string `json:"subscriptions_url"`
	OrganizationsUrl  string `json:"organizations_url"`
	ReposUrl          string `json:"repos_url"`
	EventsUrl         string `json:"events_url"`
	ReceivedEventsUrl string `json:"received_events_url"`
	Type              string `json:"type"`
	SiteAdmin         bool   `json:"site_admin"`
}

type GithubAsset struct {
	Url                string        `json:"url"`
	Id                 int           `json:"id"`
	NodeId             string        `json:"node_id"`
	Name               string        `json:"name"`
	Label              string        `json:"label"`
	Uploader           *GithubAuthor `json:"uploader"`
	ContentType        string        `json:"content_type"`
	State              string        `json:"state"`
	Size               int           `json:"size"`
	DownloadCount      int           `json:"download_count"`
	CreatedAt          string        `json:"created_at"`
	UpdatedAt          string        `json:"updated_at"`
	BrowserDownloadUrl string        `json:"browser_download_url"`
}

type GithubReactions struct {
	Url        string `json:"url"`
	TotalCount int    `json:"total_count"`
	Like       int    `json:"+1"`
	Dislike    int    `json:"-1"`
	Laugh      int    `json:"laugh"`
	Hooray     int    `json:"hooray"`
	Confused   int    `json:"confused"`
	Heart      int    `json:"heart"`
	Rocket     int    `json:"rocket"`
	Eyes       int    `json:"eyes"`
}

type GithubRelease struct {
	Url             string           `json:"url"`
	AssetsUrl       string           `json:"assets_url"`
	HtmlUrl         string           `json:"html_url"`
	Id              int              `json:"id"`
	Author          *GithubAuthor    `json:"author"`
	NodeId          string           `json:"node_id"`
	TagName         string           `json:"tag_name"`
	TargetCommitish string           `json:"target_commitish"`
	Name            string           `json:"name"`
	Draft           bool             `json:"draft"`
	Prerelease      bool             `json:"prerelease"`
	CreatedAt       string           `json:"created_at"`
	PublishedAt     string           `json:"published_at"`
	Assets          []*GithubAsset   `json:"assets"`
	TarballUrl      string           `json:"tarball_url"`
	ZipballUrl      string           `json:"zipball_url"`
	Body            string           `json:"body"`
	Reactions       *GithubReactions `json:"reactions"`
}

func GetLatestRelease() (*GithubRelease, error) {
	resp, err := http.Get("https://api.github.com/repos/Intelblox/Multiblox/releases/latest")
	if err != nil {
		fmt.Printf("Error fetching latest Github release: %s\n", err)
		return nil, err
	}
	if resp.StatusCode == 404 {
		fmt.Printf("Could not find a release.\n")
		return nil, errors.New("could not find a release")
	}
	var release *GithubRelease
	err = json.NewDecoder(resp.Body).Decode(&release)
	resp.Body.Close()
	if err != nil {
		fmt.Printf("Error converting response to JSON: %s\n", err)
		return nil, err
	}
	return release, nil
}

func GetInstaller(release *GithubRelease) (*GithubAsset, error) {
	var installerAsset *GithubAsset
	for _, asset := range release.Assets {
		if asset.Name == "MbxInstaller.exe" {
			installerAsset = asset
			break
		}
	}
	if installerAsset == nil {
		err := errors.New("not found")
		fmt.Printf("Error finding release binary: %s\n", err)
		return nil, err
	}
	return installerAsset, nil
}

func InstallMultiblox(installer *GithubAsset) error {
	if installer == nil {
		release, err := GetLatestRelease()
		if err != nil {
			return err
		}
		installer, err = GetInstaller(release)
		if err != nil {
			return err
		}
		fmt.Printf("Got latest release from repository.\n")
	}
	installerPath := filepath.Join(os.TempDir(), installer.Name)
	_, err := os.Stat(installerPath)
	if err == nil {
		err = os.Remove(installerPath)
		if err != nil {
			fmt.Printf("Error removing existing installer: %s\n", err)
			return err
		}
		fmt.Printf("Removed existing installer.\n")
	}
	installerFile, err := os.Create(installerPath)
	if err != nil {
		fmt.Printf("Error creating binary file: %s\n", err)
		return err
	}
	defer installerFile.Close()
	fmt.Printf("Created installer in temp directory.\n")
	resp, err := http.Get(installer.BrowserDownloadUrl)
	if err != nil {
		fmt.Printf("Error downloading binary file: %s\n", err)
		return err
	}
	fmt.Printf("Download started.\n")
	_, err = io.Copy(installerFile, resp.Body)
	resp.Body.Close()
	installerFile.Close()
	if err != nil {
		fmt.Printf("Error writing binary file: %s\n", err)
		return err
	}
	fmt.Printf("Wrote installer to temp directory.\n")
	cmd := exec.Command(installerPath, "/y")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	err = cmd.Start()
	if err != nil {
		fmt.Printf("Error starting binary file: %s\n", err)
		return err
	}
	fmt.Printf("Started installer.\n")
	return err
}
