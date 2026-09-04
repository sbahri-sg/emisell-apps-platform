package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type configuration struct {
	AppID, ClientID, ClientSecret, PublicURL, GatewayURL, ConsentURL, ConnectedAppsURL string
	StoreFile, EncryptionKeyFile, ListenAddress                                        string
	LocalDevelopment                                                                   bool
}

func validateOrigin(raw string, local bool) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return false
	}
	loopback := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"
	return u.Scheme == "https" || (local && u.Scheme == "http" && loopback)
}

func loadConfiguration(path string) (configuration, error) {
	var config configuration
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &config) != nil {
		return config, fmt.Errorf("cannot load private Product Reader configuration")
	}
	if !validateOrigin(config.PublicURL, config.LocalDevelopment) || !validateOrigin(config.GatewayURL, config.LocalDevelopment) ||
		!opaqueID.MatchString(config.AppID) || config.ClientID == "" || config.ClientSecret == "" {
		return config, fmt.Errorf("invalid Product Reader origins or app credentials")
	}
	for _, raw := range []string{config.ConsentURL, config.ConnectedAppsURL} {
		u, err := url.Parse(raw)
		if err != nil || u.RawQuery != "" || u.Fragment != "" {
			return config, fmt.Errorf("invalid platform URL")
		}
		origin := *u
		origin.Path = ""
		if !validateOrigin(origin.String(), config.LocalDevelopment) {
			return config, fmt.Errorf("invalid platform origin")
		}
	}
	if config.LocalDevelopment {
		host, _, err := net.SplitHostPort(config.ListenAddress)
		if err != nil || (host != "127.0.0.1" && host != "::1") {
			return config, fmt.Errorf("local example must bind loopback")
		}
	}
	return config, nil
}

func main() {
	config, err := loadConfiguration(os.Getenv("PRODUCT_READER_CONFIG_FILE"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	key, err := os.ReadFile(config.EncryptionKeyFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot read Product Reader encryption key")
		os.Exit(1)
	}
	storage, err := openStore(config.StoreFile, key)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot open encrypted Product Reader store")
		os.Exit(1)
	}
	defer storage.Close()
	reader := newReader(config, storage)
	server := &http.Server{Addr: config.ListenAddress, Handler: reader.handler(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 * 1024}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	// Never log URLs containing query strings, credentials, authorization codes or tokens.
	fmt.Println("Product Reader: " + strings.TrimSuffix(config.PublicURL, "/") + "/")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, "Product Reader server stopped unexpectedly")
		os.Exit(1)
	}
}
