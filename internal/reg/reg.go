package reg

import (
	"github.com/Intelblox/Multiblox/internal/app"
	"golang.org/x/sys/windows/registry"
)

func Get(key string) (string, error) {
	appKey, err := registry.OpenKey(registry.CURRENT_USER, app.ConfigKey, registry.ALL_ACCESS)
	if err != nil {
		return "", err
	}
	rcv, _, err := appKey.GetStringValue(key)
	appKey.Close()
	if err != nil {
		return "", err
	}
	return rcv, nil
}
