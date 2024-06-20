package httputil

import (
	"encoding/json"
	"net/http"
)

func GetJson(url string, v any) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&v)
	if err != nil {
		return err
	}
	return nil
}
