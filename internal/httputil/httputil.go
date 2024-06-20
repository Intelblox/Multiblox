package httputil

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func GetJson(url string, v any) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get response: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&v)
	if err != nil {
		return fmt.Errorf("failed to parse json: %s", err)
	}
	return nil
}
