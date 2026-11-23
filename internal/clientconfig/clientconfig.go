// Package clientconfig resolves settings for the CLI half of Mako.
//
// The proxy itself requires every setting to come from the environment with no
// fallback, because a misconfigured server should fail loudly at startup. The
// CLI is a local developer tool, so it defaults to the local proxy and lets a
// flag or the environment override it.
package clientconfig

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// osGetenv is a variable so tests can stub the environment lookup.
var osGetenv = os.Getenv

const defaultProxyURL = "http://localhost:8080"

// override is set from the --proxy flag and wins over the environment.
var override string

// SetProxyURL applies a command-line override. An empty value is ignored, so
// callers can pass an unset flag straight through.
func SetProxyURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	normalised, err := NormaliseProxyURL(raw)
	if err != nil {
		return err
	}
	override = normalised
	return nil
}

// ProxyURL returns the base URL the CLI should talk to.
// Precedence: --proxy flag, then MAKO_PROXY_URL, then the local default.
func ProxyURL() string {
	if override != "" {
		return override
	}
	if v := envProxyURL(); v != "" {
		if normalised, err := NormaliseProxyURL(v); err == nil {
			return normalised
		}
		return v
	}
	return defaultProxyURL
}

// NormaliseProxyURL cleans up a user-supplied proxy address.
//
// A tunnel host is usually pasted without a scheme ("abc123.ngrok-free.app"),
// which would otherwise be parsed as a relative path and produce a confusing
// connection error much later. A bare host is assumed to be HTTPS, since that is
// what every tunnelling service hands out.
func NormaliseProxyURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("proxy URL is empty")
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("proxy URL %q is not valid: %w", raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("proxy URL %q must use http or https", raw)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("proxy URL %q has no host", raw)
	}

	// Requests append their own path, so keep only scheme and host.
	return strings.TrimSuffix(parsed.Scheme+"://"+parsed.Host+parsed.Path, "/"), nil
}

// envProxyURL is split out so tests can exercise precedence without touching
// the process environment ordering.
func envProxyURL() string { return osGetenv("MAKO_PROXY_URL") }
