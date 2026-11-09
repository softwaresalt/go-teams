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

// ValidateMeetingURL reports whether raw is a non-empty absolute HTTP(S) URL
// with a host. It is the single source of truth for meeting-URL validation,
// reused by MutateJoinURL and by the browser package before navigation.
func ValidateMeetingURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("teams: empty meeting url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("teams: parse meeting url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("teams: meeting url must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("teams: meeting url must have a host")
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
