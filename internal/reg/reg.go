package reg

import (
	"github.com/Intelblox/Multiblox/internal/app"
	"golang.org/x/sys/windows/registry"
)

func GetRobloxClientVersion() (string, error) {
	appk, err := registry.OpenKey(registry.CURRENT_USER, app.ConfigKey, registry.ALL_ACCESS)
	if err != nil {
		return "", err
	}
	rcv, _, err := appk.GetStringValue("RobloxClientVersion")
	if err != nil {
		return "", err
	}
	return rcv, nil
}
