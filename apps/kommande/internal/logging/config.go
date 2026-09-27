package logging

import (
	"net/url"
	"strings"
)

// RedactURI returns uri with any password replaced by xxxxx, so a connection
// string can be logged. An unparseable value is returned unchanged, keeping a
// malformed setting visible without risking a panic at boot.
func RedactURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}
	if _, has := u.User.Password(); has {
		u.User = url.UserPassword(u.User.Username(), "xxxxx")
	}
	return u.String()
}

// Set reports whether a secret is configured, without revealing any of it.
func Set(v string) string {
	if strings.TrimSpace(v) == "" {
		return "<empty>"
	}
	return "<set>"
}
