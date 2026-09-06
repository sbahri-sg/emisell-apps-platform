package localfiles

import (
	"bytes"
	"crypto/rand"
	"emisell.app/platform/internal/event/natsbus"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
)

type EventConfig struct {
	Worker natsbus.Credentials `json:"worker"`
	Core   natsbus.Credentials `json:"core"`
}

// LocalBrokerConfig creates no public/default password and grants the reference
// consumer only its pre-provisioned tenant-filtered durable (not stream APIs).
func LocalBrokerConfig() (EventConfig, []byte) {
	workerPass, corePass := rand.Text()+rand.Text(), rand.Text()+rand.Text()
	c := EventConfig{
		Worker: natsbus.Credentials{URL: "nats://127.0.0.1:54227", User: "outbox-worker", Password: workerPass, Inbox: "_INBOX.worker"},
		Core:   natsbus.Credentials{URL: "nats://127.0.0.1:54227", User: "core-local-store", Password: corePass, Inbox: "_INBOX.core_local_store"},
	}
	config := map[string]any{
		"port": 4222, "max_payload": 32768, "jetstream": map[string]any{"store_dir": "/data", "max_memory_store": 64 << 20, "max_file_store": 1 << 30},
		"authorization": map[string]any{"users": []any{
			map[string]any{"user": c.Worker.User, "password": workerPass, "permissions": map[string]any{"publish": []string{"$JS.API.>", "emisell.events.>", "$JS.ACK.EMISELL_EVENTS.webhook_router.>"}, "subscribe": []string{"_INBOX.worker.>"}}},
			map[string]any{"user": c.Core.User, "password": corePass, "permissions": map[string]any{"publish": []string{"$JS.API.CONSUMER.INFO.EMISELL_EVENTS.core_local-store", "$JS.API.CONSUMER.MSG.NEXT.EMISELL_EVENTS.core_local-store", "$JS.ACK.EMISELL_EVENTS.core_local-store.>"}, "subscribe": []string{"_INBOX.core_local_store.>"}}},
		}},
	}
	raw, _ := json.MarshalIndent(config, "", "  ")
	raw = bytes.ReplaceAll(raw, []byte(`\u003e`), []byte(">"))
	return c, raw
}
func InitEvents() error {
	var current EventConfig
	err := Read(".local/events.json", &current)
	if err == nil {
		if _, err = os.Stat(".local/nats.conf"); err != nil {
			return fmt.Errorf("broker config missing; preserve credentials and repair explicitly")
		}
		raw, err := os.ReadFile(".local/nats.conf")
		if err != nil {
			return err
		}
		// NATS config syntax rejects JSON's HTML escape for subject wildcards.
		fixed := bytes.ReplaceAll(raw, []byte(`\u003e`), []byte(">"))
		var config struct {
			Authorization struct {
				Users []struct {
					User        string `json:"user"`
					Permissions struct {
						Publish []string `json:"publish"`
					} `json:"permissions"`
				} `json:"users"`
			} `json:"authorization"`
		}
		if err = json.Unmarshal(fixed, &config); err != nil {
			return err
		}
		for _, user := range config.Authorization.Users {
			if user.User == current.Worker.User && !slices.Contains(user.Permissions.Publish, "$JS.ACK.EMISELL_EVENTS.webhook_router.>") {
				// Preserve credentials and every existing permission; extend only this worker's ACK subject.
				var data map[string]any
				if err = json.Unmarshal(fixed, &data); err != nil {
					return err
				}
				for _, entry := range data["authorization"].(map[string]any)["users"].([]any) {
					u := entry.(map[string]any)
					if u["user"] == current.Worker.User {
						p := u["permissions"].(map[string]any)
						p["publish"] = append(p["publish"].([]any), "$JS.ACK.EMISELL_EVENTS.webhook_router.>")
					}
				}
				fixed, err = json.MarshalIndent(data, "", "  ")
				if err != nil {
					return err
				}
				fixed = bytes.ReplaceAll(fixed, []byte(`\u003e`), []byte(">"))
			}
		}
		if !bytes.Equal(raw, fixed) {
			return WriteBytes(".local/nats.conf", fixed, true)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cfg, raw := LocalBrokerConfig()
	if err = WriteBytes(".local/nats.conf", raw, false); err != nil {
		return err
	}
	return Write(".local/events.json", cfg, false)
}
