package localfiles

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
)

type RemoteConfig struct {
	Origin           string `json:"origin"`
	AuthorizationURL string `json:"authorizationUrl"`
	CallbackURL      string `json:"callbackUrl"`
	ClientID         string `json:"clientId"`
	ClientSecret     string `json:"clientSecret"`
	EncryptionKey    string `json:"encryptionKey"`
}

func key32() string {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return base64.RawStdEncoding.EncodeToString(key)
}
func NewRemoteConfigs() (RemoteConfig, RemoteConfig) {
	client := RemoteConfig{Origin: "http://127.0.0.1:8091", AuthorizationURL: "http://localhost:8091/oauth/authorize", CallbackURL: "http://localhost:4317/api/v1/oauth/callback", ClientID: "emisell-local-client", ClientSecret: rand.Text() + rand.Text(), EncryptionKey: key32()}
	app := client
	app.EncryptionKey = key32()
	return client, app
}
func InitRemoteFiles() error {
	var current, app RemoteConfig
	err := Read(".local/remote-platform.json", &current)
	if err == nil {
		if err = Read(".local/remote-app.json", &app); err != nil {
			return err
		}
		if current.ClientSecret != app.ClientSecret || current.ClientID != app.ClientID {
			return errors.New("remote configuration mismatch")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	client, app := NewRemoteConfigs()
	if err = Write(".local/remote-platform.json", client, false); err != nil {
		return err
	}
	return Write(".local/remote-app.json", app, false)
}
