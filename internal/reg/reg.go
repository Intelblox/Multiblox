package reg

import (
	"path/filepath"

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

func DeleteKey(k registry.Key, path string) error {
	key, err := registry.OpenKey(k, path, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer key.Close()
	names, _ := key.ReadSubKeyNames(0)
	for _, name := range names {
		err = DeleteKey(k, filepath.Join(path, name))
		if err != nil {
			return err
		}
	}
	err = registry.DeleteKey(k, path)
	if err != nil {
		return err
	}
	return nil
}
