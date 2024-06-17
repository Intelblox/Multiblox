package regconf

import "golang.org/x/sys/windows/registry"

func GetRobloxClientVersion() (string, error) {
	appk, err := registry.OpenKey(registry.CURRENT_USER, "SOFTWARE\\Intelblox Foundation\\Multiblox", registry.ALL_ACCESS)
	if err != nil {
		return "", err
	}
	rcv, _, err := appk.GetStringValue("RobloxClientVersion")
	if err != nil {
		return "", err
	}
	return rcv, nil
}

func SetRobloxClientVersion(version string) error {
	appk, _, err := registry.CreateKey(registry.CURRENT_USER, "SOFTWARE\\Intelblox Foundation\\Multiblox", registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	err = appk.SetStringValue("RobloxClientVersion", version)
	if err != nil {
		return err
	}
	return nil
}
