package localfiles

import (
	"emisell.app/platform/internal/platform/secretbox"
	"errors"
	"os"
)

const applicationKeyFile = ".local/application-credentials.json"

func ReadApplicationCredentialBox() (*secretbox.Box, error) {
	key := os.Getenv("EMISELL_APP_CREDENTIAL_KEY")
	if key == "" {
		if os.Getenv("EMISELL_ENV") == "production" {
			return nil, errors.New("EMISELL_APP_CREDENTIAL_KEY is required")
		}
		var cfg struct {
			Key string `json:"key"`
		}
		if err := Read(applicationKeyFile, &cfg); err != nil {
			return nil, err
		}
		key = cfg.Key
	}
	box, err := secretbox.New(key)
	if err != nil {
		return nil, err
	}
	return &box, nil
}
func InitApplicationCredentialKey() error {
	_, err := ReadApplicationCredentialBox()
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return Write(applicationKeyFile, struct {
		Key string `json:"key"`
	}{key32()}, false)
}
