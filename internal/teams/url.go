// Package teams provides helpers for interacting with Microsoft Teams meeting
// URLs.
package teams

import (
	"fmt"
	"net/url"
	"regexp"
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
		return "", fmt.Errorf("teams: meeting url is not parseable")
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

// invalidURLPlaceholder is returned by RedactURL when a safe scheme://host/path
// form cannot be guaranteed.
const invalidURLPlaceholder = "[invalid-url]"

// RedactURL returns raw reduced to scheme://host/path so it can be logged or
// embedded in errors without leaking userinfo credentials, query strings such
// as the "p" passcode or OAuth codes, or fragments such as access tokens.
// When raw is malformed, opaque, hostless, or not HTTP(S) and a clean form free
// of '@', '?', '#', and control characters cannot be guaranteed, RedactURL
// returns "[invalid-url]".
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" && u.Opaque == "" {
		u.User = nil
		u.RawQuery = ""
		u.ForceQuery = false
		u.Fragment = ""
		u.RawFragment = ""
		u.Opaque = ""
		return u.String()
	}
	return redactMalformedURL(raw)
}

// redactMalformedURL is the conservative fallback for URLs that url.Parse
// cannot represent safely. It strips any userinfo (everything through the last
// '@' before the first '/'), the query, and the fragment, and returns the
// placeholder when what remains is not a clean scheme://host[:port]/path.
func redactMalformedURL(raw string) string {
	lower := strings.ToLower(raw)
	var scheme string
	switch {
	case strings.HasPrefix(lower, "https://"):
		scheme = "https"
	case strings.HasPrefix(lower, "http://"):
		scheme = "http"
	default:
		return invalidURLPlaceholder
	}
	rest := raw[len(scheme)+len("://"):]
	// The authority ends at the first '/', '?', or '#', so the query and
	// fragment are removed before the host is separated from the path: a
	// secret containing '/' in either must never be promoted into the path.
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest = rest[:i]
	}
	head, tail := rest, ""
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		head, tail = rest[:i], rest[i:]
	}
	if at := strings.LastIndexByte(head, '@'); at >= 0 {
		head = head[at+1:]
	}
	if !isCleanHostPort(head) {
		return invalidURLPlaceholder
	}
	out := scheme + "://" + head + tail
	for _, r := range out {
		if r == '@' || r == '?' || r == '#' || r <= 0x20 || r == 0x7f {
			return invalidURLPlaceholder
		}
	}
	return out
}

// isCleanHostPort reports whether s is a plausible host with an optional
// all-numeric port. A non-numeric "port" may be a user:password pair, so it is
// rejected rather than echoed.
func isCleanHostPort(s string) bool {
	if s == "" {
		return false
	}
	host := s
	bracketed := false
	if strings.HasPrefix(s, "[") {
		end := strings.IndexByte(s, ']')
		if end < 0 {
			return false
		}
		bracketed = true
		host = s[:end+1]
		if port := s[end+1:]; port != "" && !isNumericPort(port) {
			return false
		}
	} else if i := strings.LastIndexByte(s, ':'); i >= 0 {
		host = s[:i]
		if !isNumericPort(s[i:]) {
			return false
		}
	}
	if host == "" || strings.ContainsAny(host, "%@ \t") {
		return false
	}
	return bracketed || !strings.Contains(host, ":")
}

// isNumericPort reports whether s is ":" followed by one or more digits.
func isNumericPort(s string) bool {
	if len(s) < 2 || s[0] != ':' {
		return false
	}
	for _, r := range s[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// RedactArgs joins args with spaces for logging, replacing every embedded
// HTTP(S) URL (bare, "--flag=URL", or attached "-uURL") with its RedactURL form.
func RedactArgs(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = redactArg(a)
	}
	return strings.Join(out, " ")
}

// httpSchemeTokenRe finds the earliest http: or https: token in an argument,
// including malformed delimiters such as "https:/host", so everything from the
// outermost URL onward is redacted as one unit.
var httpSchemeTokenRe = regexp.MustCompile(`(?i)https?:`)

func redactArg(a string) string {
	loc := httpSchemeTokenRe.FindStringIndex(a)
	i := strings.Index(a, "://")
	var start int
	switch {
	case loc == nil && i < 0:
		return a
	case i < 0:
		start = loc[0]
	default:
		start = i
		for start > 0 && isASCIILetter(a[start-1]) {
			start--
		}
		// An http(s) token that is itself the scheme of the first "://" may be
		// glued to a flag ("-uhttps://"), and an http(s) token that precedes
		// the first "://" belongs to an earlier, malformed URL. Both start at
		// the http token; otherwise the first "://" scheme starts the URL.
		if loc != nil && (loc[0] < start || loc[1] == i+1) {
			start = loc[0]
		}
	}
	return a[:start] + RedactURL(a[start:])
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
