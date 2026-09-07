package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// PublicOrigins is deployment configuration, never inferred from forwarded headers.
type PublicOrigins struct {
	Admin, Developer, Store string
	Secure                  bool
}

func ReadPublicOrigins() (PublicOrigins, error) {
	c := PublicOrigins{Admin: os.Getenv("EMISELL_ADMIN_ORIGIN"), Developer: os.Getenv("EMISELL_DEVELOPER_ORIGIN"), Store: os.Getenv("EMISELL_STORE_ORIGIN")}
	if dashboard := os.Getenv("EMISELL_DASHBOARD_ORIGIN"); dashboard != "" {
		if c.Admin != "" || c.Developer != "" {
			return c, fmt.Errorf("use dashboard origin without legacy admin/developer origins")
		}
		c.Admin = dashboard
		c.Developer = dashboard
	}
	if c.Admin == "" && c.Developer == "" && c.Store == "" && os.Getenv("EMISELL_ENV") != "production" {
		return PublicOrigins{Admin: "http://localhost:4317", Developer: "http://localhost:4319", Store: "http://localhost:4318"}, nil
	}
	hosts := map[string]bool{}
	for index, origin := range []string{c.Admin, c.Developer, c.Store} {
		if index == 2 && origin == "" && os.Getenv("EMISELL_STORE_DISABLED") == "true" && os.Getenv("EMISELL_DASHBOARD_ORIGIN") != "" {
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.User != nil || u.Opaque != "" || u.Port() != "" {
			return c, fmt.Errorf("all three public origins must be HTTPS domain origins without path, port, credentials or query")
		}
		host := u.Hostname()
		if host != strings.ToLower(host) || net.ParseIP(host) != nil || !strings.Contains(host, ".") || strings.HasSuffix(host, ".") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
			return c, fmt.Errorf("public origins require distinct public domain names")
		}
		for _, label := range strings.Split(host, ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return c, fmt.Errorf("invalid public domain label")
			}
			for _, ch := range label {
				if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
					return c, fmt.Errorf("invalid public domain character")
				}
			}
		}
		if hosts[host] {
			if index == 1 && os.Getenv("EMISELL_DASHBOARD_ORIGIN") != "" && c.Developer == c.Admin {
				continue
			}
			return c, fmt.Errorf("Admin, Developer and Store must use distinct domains")
		}
		hosts[host] = true
	}
	c.Secure = true
	return c, nil
}
func (c PublicOrigins) ForSurface(surface string) string {
	if surface == "developer" {
		return c.Developer
	}
	return c.Admin
}
func (c PublicOrigins) AllowsHost(host string) bool {
	for _, origin := range []string{c.Admin, c.Developer, c.Store} {
		u, err := url.Parse(origin)
		if err == nil && host == u.Host {
			return true
		}
	}
	return false
}
