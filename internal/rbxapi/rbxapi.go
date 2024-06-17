package rbxapi

import (
	"encoding/json"
	"fmt"
	"net/http"
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
