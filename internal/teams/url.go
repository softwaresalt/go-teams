// Package teams provides helpers for interacting with Microsoft Teams meeting
// URLs.
package teams

import (
	"fmt"
	"net/url"
	"strings"
)

// bypassParams are the query parameters appended to a legacy Teams
// /l/meetup-join URL to force the web client to render the meeting SPA
// directly, bypassing the "open in the desktop app" interstitial.
var bypassParams = map[string]string{
	"msLaunch":         "false",
	"type":             "meetup-join",
	"directDl":         "true",
	"enableMobilePage": "true",
	"suppressPrompt":   "true",
}

// trustedHostSuffixes lists the Microsoft 365 / Teams / SharePoint host
// suffixes a remote meeting URL may target. A host matches when it equals a
// suffix or is a dot-delimited subdomain of it.
var trustedHostSuffixes = []string{
	"teams.microsoft.com",
	"teams.live.com",
	"teams.cloud.microsoft",
	"sharepoint.com",
	"microsoft.com",
	"office.com",
	"sfx.ms",
	"aka.ms",
}

// isLoopbackHost reports whether host (as returned by url.URL.Hostname) is a
// loopback name used by local test fixtures.
func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// isTrustedRemoteHost reports whether host equals or is a dot-delimited
// subdomain of a trusted Microsoft 365 host suffix.
func isTrustedRemoteHost(host string) bool {
	host = strings.ToLower(host)
	for _, suffix := range trustedHostSuffixes {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// ValidateMeetingURL reports whether raw is an acceptable initial meeting URL.
// It requires an absolute HTTP(S) URL with a host and no embedded userinfo.
// Loopback hosts (localhost, 127.0.0.1, ::1) may use http or https so local
// test fixtures keep working; every other host must use https and match a
// trusted Microsoft 365 / Teams / SharePoint host suffix. It is the single
// source of truth for meeting-URL validation, reused by MutateJoinURL and by
// the browser package before browser allocation. Error messages never echo
// the raw URL, so passcodes and credentials are not leaked.
//
// Validation covers initial-URL ingress only; redirects followed inside the
// browser after navigation are a documented residual risk.
func ValidateMeetingURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("teams: empty meeting url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("teams: meeting url is not parseable")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("teams: meeting url must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("teams: meeting url must have a host")
	}
	if u.User != nil {
		return fmt.Errorf("teams: meeting url must not embed credentials")
	}
	host := u.Hostname()
	if isLoopbackHost(host) {
		return nil
	}
	if u.Scheme != "https" {
		return fmt.Errorf("teams: meeting url for host %q must use https", host)
	}
	if !isTrustedRemoteHost(host) {
		return fmt.Errorf("teams: meeting url host %q is not a trusted Microsoft Teams host", host)
	}
	return nil
}

// MutateJoinURL normalizes a Teams meeting URL for browser navigation. For
// legacy /l/meetup-join links it appends the web-client bypass parameters; for
// modern /meet/ links (and other non-legacy paths) it strips any conflicting
// legacy "type" parameter while preserving the path, passcode token ("p"),
// unrelated query parameters, and URL fragment.
func MutateJoinURL(raw string) (string, error) {
	if err := ValidateMeetingURL(raw); err != nil {
		return "", err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("teams: parse meeting url: %w", err)
	}
	q := u.Query()
	if isLegacyMeetupJoinPath(u.Path) {
		for k, v := range bypassParams {
			q.Set(k, v)
		}
	} else {
		q.Del("type")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func isLegacyMeetupJoinPath(path string) bool {
	return path == "/l/meetup-join" || strings.HasPrefix(path, "/l/meetup-join/")
}
