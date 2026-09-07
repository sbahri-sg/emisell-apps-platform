package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
)

const LocalDatabase = "postgres://emisell_local:local-development-only@127.0.0.1:55437/emisell_local?sslmode=disable"

type Config struct{ Address, Origin, DatabaseURL, RPCAddress string }

func Read() (Config, error) {
	c := Config{Address: "127.0.0.1:8087", Origin: "http://localhost:4317", DatabaseURL: LocalDatabase, RPCAddress: "127.0.0.1:8088"}
	if _, err := ReadPublicOrigins(); err != nil {
		return c, err
	}
	if os.Getenv("EMISELL_ENV") == "production" {
		origins, _ := ReadPublicOrigins()
		c.Origin = origins.Admin
		c.Address = "0.0.0.0:8087"
		c.DatabaseURL = os.Getenv("EMISELL_DATABASE_URL")
		db, err := url.Parse(c.DatabaseURL)
		if err != nil || (db.Scheme != "postgres" && db.Scheme != "postgresql") || db.Hostname() == "" || db.User == nil || db.User.Username() == "" || db.Path == "" || db.Path == "/" || db.Fragment != "" {
			return c, fmt.Errorf("explicit production PostgreSQL URL required")
		}
		password, ok := db.User.Password()
		if !ok || len(password) < 16 || password == "local-development-only" {
			return c, fmt.Errorf("production database requires a non-default password of at least 16 characters")
		}
		// RPC remains loopback-only until the production engine path is certified.
		return c, nil
	}
	if v := os.Getenv("EMISELL_RPC_ADDRESS"); v != "" {
		c.RPCAddress = v
	}
	rpcHost, _, rpcErr := net.SplitHostPort(c.RPCAddress)
	if rpcErr != nil || net.ParseIP(rpcHost) == nil || !net.ParseIP(rpcHost).IsLoopback() {
		return c, fmt.Errorf("internal RPC must bind to a loopback IP")
	}
	if v := os.Getenv("EMISELL_ADDRESS"); v != "" {
		c.Address = v
	}
	if v := os.Getenv("EMISELL_ORIGIN"); v != "" {
		c.Origin = v
	}
	if v := os.Getenv("EMISELL_DATABASE_URL"); v != "" {
		c.DatabaseURL = v
	}
	host, _, err := net.SplitHostPort(c.Address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return c, fmt.Errorf("local simulator must bind to a loopback IP")
	}
	u, err := url.Parse(c.Origin)
	if err != nil || u.Scheme != "http" || u.Path != "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !(u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1") {
		return c, fmt.Errorf("local origin must use http localhost or 127.0.0.1")
	}
	db, err := url.Parse(c.DatabaseURL)
	if err != nil || !(db.Hostname() == "127.0.0.1" || db.Hostname() == "localhost") || db.Path != "/emisell_local" {
		return c, fmt.Errorf("local database must be named emisell_local on loopback")
	}
	return c, nil
}
