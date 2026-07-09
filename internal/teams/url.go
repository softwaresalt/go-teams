// Package teams provides helpers for interacting with Microsoft Teams meeting
// URLs.
package teams

import (
	"fmt"
	"net/url"
)

// bypassParams are the query parameters appended to a Teams meeting URL to
// force the web client to render the meeting SPA directly, bypassing the
// "open in the desktop app" interstitial.
var bypassParams = map[string]string{
	"msLaunch":         "false",
	"type":             "meetup-join",
	"directDl":         "true",
	"enableMobilePage": "true",
	"suppressPrompt":   "true",
}

// MutateJoinURL appends the Teams web-client bypass parameters to raw,
// preserving any existing query parameters. It returns an error if raw is not
// a valid absolute HTTP(S) URL.
func MutateJoinURL(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("teams: empty meeting url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("teams: parse meeting url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("teams: meeting url must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("teams: meeting url must have a host")
	}
	q := u.Query()
	for k, v := range bypassParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
