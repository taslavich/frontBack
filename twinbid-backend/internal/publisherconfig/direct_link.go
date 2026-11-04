package publisherconfig

import (
	"errors"
	"net/url"
	"strings"
)

// DirectLinkOrigin falls back to the existing Publisher public origin.
func (c Config) DirectLinkOrigin() string {
	if c.DirectLinkPublicURL != "" {
		return strings.TrimRight(c.DirectLinkPublicURL, "/")
	}
	return strings.TrimRight(c.PublicURL, "/")
}

func (c Config) validateDirectLinkOrigin() error {
	raw := c.DirectLinkPublicURL
	if raw == "" {
		raw = c.PublicURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Opaque != "" ||
		(u.Path != "" && u.Path != "/") || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") {
		return errors.New("PUBLISHER_DIRECT_LINK_PUBLIC_URL (or its PUBLISHER_PUBLIC_URL fallback) must be a bare HTTPS origin")
	}
	return nil
}
