package webhookconfig

import (
	"emisell.app/platform/pkg/accessscope"
	"testing"
)

func TestVersionedConfiguration(t *testing.T) {
	scopes := &accessscope.Declaration{Profile: accessscope.Profile, Required: []string{"read_products"}, Optional: []string{"read_orders"}}
	config := func() Config {
		return Config{APIVersion: APIVersion, Subscriptions: []Subscription{{Topics: []string{"products.created", "orders.updated"}, URI: "https://example.com/events"}}}
	}
	c := config()
	if err := c.Validate(scopes); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Config){
		func(c *Config) { c.APIVersion = "1.0.0" },
		func(c *Config) { c.Subscriptions[0].Topics = []string{"*"} },
		func(c *Config) { c.Subscriptions[0].Topics = []string{"fulfillments.updated"} },
		func(c *Config) { c.Subscriptions[0].Topics = []string{"products.created", "products.created"} },
		func(c *Config) { c.Subscriptions[0].URI = "http://example.com" },
		func(c *Config) { c.Subscriptions[0].Topics = nil },
		func(c *Config) { c.Subscriptions = nil },
	} {
		c := config()
		mutate(&c)
		if c.Validate(scopes) == nil {
			t.Fatal("invalid configuration accepted", c)
		}
	}
	if c.Validate(nil) == nil {
		t.Fatal("missing scopes accepted")
	}
	empty := Config{APIVersion: APIVersion, Subscriptions: []Subscription{}}
	if empty.Validate(nil) != nil {
		t.Fatal("cannot explicitly remove configuration")
	}
}
