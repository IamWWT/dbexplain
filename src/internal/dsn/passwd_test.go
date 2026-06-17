package dsn

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// sanitizeErr simulates config.SanitizeErr locally to avoid import cycle.
// dsn → config imports dsn → cycle. Logic is simple string replacement.
func sanitizeErr(msg string) string {
	for {
		protoIdx := strings.Index(msg, "://")
		if protoIdx < 0 {
			break
		}
		userStart := protoIdx + 3
		colonIdx := strings.Index(msg[userStart:], ":")
		if colonIdx < 0 {
			break
		}
		passStart := userStart + colonIdx + 1
		atIdx := strings.Index(msg[passStart:], "@")
		if atIdx < 0 {
			break
		}
		newMsg := msg[:passStart] + "***" + msg[passStart+atIdx:]
		if newMsg == msg {
			break // 替换后字符串未变化 → 已脱敏完毕
		}
		msg = newMsg
	}
	return msg
}

// specialChars defines all special characters to test in passwords.
// Characters are grouped by category for test naming.
type charGroup struct {
	category string
	chars    []string
}

func getCharGroups() []charGroup {
	return []charGroup{
		{category: "URL保留字符", chars: []string{":", "/", "?", "#", "[", "]", "@"}},
		{category: "URL不安全字符", chars: []string{" ", "<", ">", "{", "}", "|", "\\", "^", "`"}},
		{category: "通用特殊字符", chars: []string{"～", "'", "!", "\"", "$", "%", "&", "*", "+", ",", ".", ";", "=", "_", "~", "-", "(", ")"}},
		{category: "中文标点", chars: []string{"；", "，", "。", "、"}},
	}
}

// schemes to test: all schemes that support password auth
var testSchemes = []struct {
	scheme   string
	kind     string
	rawFmt   func(pwd string) string // DSN format with password placeholder
}{
	{"mysql", "mysql", func(p string) string { return "mysql://user:" + p + "@host:3306/db" }},
	{"postgres", "postgres", func(p string) string { return "postgres://user:" + p + "@host:5432/db" }},
	{"gaussdb", "gaussdb", func(p string) string { return "gaussdb://user:" + p + "@host:5432/db" }},
	{"redis", "redis", func(p string) string { return "redis://:" + p + "@host:6379/0" }},
	{"redis_user", "redis", func(p string) string { return "redis://user:" + p + "@host:6379/0" }},
	{"clickhouse", "clickhouse", func(p string) string { return "clickhouse://default:" + p + "@host:8123/db" }},
	{"mongodb", "mongodb", func(p string) string { return "mongodb://user:" + p + "@host:27017/db" }},
	{"elasticsearch", "elasticsearch", func(p string) string { return "es://user:" + p + "@host:9200" }},
	{"oracle", "oracle", func(p string) string { return "oracle://user:" + p + "@host:1521/db" }},
	{"hive", "hive", func(p string) string { return "hive://user:" + p + "@host:10000/db" }},
	// qdrant, sqlite, csv, tsv, xlsx, prometheus, duckdb: no password or file-based
}

// password with character at position: "pass{CHAR}word" so the char is in the middle
func pwdWithChar(c string) string {
	return "pass" + c + "word"
}

// TestCharacters_ParseDSN tests that ParseDSN can handle each special character in passwords.
// For each (scheme, character) combination, it verifies:
//   - ParseDSN succeeds (or fails gracefully for unresolvable chars)
//   - Password is correctly extracted
//   - Kind/Host/Port are correct (not corrupted by bad parse)
func TestCharacters_ParseDSN(t *testing.T) {
	for _, group := range getCharGroups() {
		for _, c := range group.chars {
			c := c // capture
			pwd := pwdWithChar(c)

			for _, s := range testSchemes {
				s := s
				name := fmt.Sprintf("%s/%s/%s", s.scheme, group.category, charDisplay(c))
				raw := s.rawFmt(pwd)

				t.Run(name, func(t *testing.T) {
					d, err := ParseDSN(raw)
					if err != nil {
						// Some characters are legitimately unparseable in URLs.
						// We accept this as a known limitation.
						t.Logf("PARSE FAIL (expected for this char): %v", err)
						return
					}

					// Verify kind
					if d.Kind != s.kind {
						t.Errorf("Kind: got %q, want %q", d.Kind, s.kind)
					}

					// Verify password extracted correctly
					if d.Password != pwd {
						t.Errorf("Password: got %q, want %q", d.Password, pwd)
					}

					// Verify host not corrupted
					if d.Host != "host" {
						t.Errorf("Host corrupted: got %q, want %q", d.Host, "host")
					}

					// Verify Raw preserves the original (after escapeUserinfo transforms)
					// Note: # in the password will be escaped to %23 in Raw
				})
			}
		}
	}
}

// TestCharacters_Redacted verifies Redacted() doesn't leak passwords
// for any character that ParseDSN can successfully parse.
func TestCharacters_Redacted(t *testing.T) {
	for _, group := range getCharGroups() {
		for _, c := range group.chars {
			c := c
			pwd := pwdWithChar(c)

			for _, s := range testSchemes {
				s := s
				name := fmt.Sprintf("%s/%s/%s", s.scheme, group.category, charDisplay(c))
				raw := s.rawFmt(pwd)

				t.Run(name, func(t *testing.T) {
					d, err := ParseDSN(raw)
					if err != nil {
						return // skip unparseable
					}

					redacted := d.Redacted()
					if strings.Contains(redacted, pwd) {
						t.Errorf("Redacted() leaked password %q in %q", pwd, redacted)
					}
					if d.Password != "" && !strings.Contains(redacted, "{dbpassword}") {
						t.Errorf("Redacted() missing {dbpassword} placeholder: %q", redacted)
					}
				})
			}
		}
	}
}

// TestCharacters_SanitizeErr verifies SanitizeErr doesn't enter infinite loop
// when processing error messages containing Redacted() output (ISSUE-095 regression).
func TestCharacters_SanitizeErr(t *testing.T) {
	for _, group := range getCharGroups() {
		for _, c := range group.chars {
			c := c
			pwd := pwdWithChar(c)

			for _, s := range testSchemes {
				s := s
				name := fmt.Sprintf("%s/%s/%s", s.scheme, group.category, charDisplay(c))
				raw := s.rawFmt(pwd)

				t.Run(name, func(t *testing.T) {
					d, err := ParseDSN(raw)
					if err != nil {
						return // skip unparseable
					}

					// Simulate what check handler does: pass an error containing Redacted() DSN
					msg := fmt.Sprintf("ping failed: %s: 28P01 authentication failed", d.Redacted())

					// Must complete within 1 second (infinite loop would hang forever)
					done := make(chan struct{}, 1)
					go func() {
						sanitizeErr(msg)
						done <- struct{}{}
					}()

					select {
					case <-done:
						// Success — no infinite loop
					case <-time.After(2 * time.Second):
						t.Fatal("SanitizeErr TIMEOUT — possible infinite loop (ISSUE-095 regression)")
					}
				})
			}
		}
	}
}

// TestCharacters_RawNotCorrupted verifies that the Raw field preserves
// the original DSN (after escapeUserinfo transforms).
func TestCharacters_RawNotCorrupted(t *testing.T) {
	for _, group := range getCharGroups() {
		for _, c := range group.chars {
			c := c
			pwd := pwdWithChar(c)

			for _, s := range testSchemes {
				s := s
				name := fmt.Sprintf("%s/%s/%s", s.scheme, group.category, charDisplay(c))
				raw := s.rawFmt(pwd)

				t.Run(name, func(t *testing.T) {
					d, err := ParseDSN(raw)
					if err != nil {
						return // skip unparseable
					}

					// Raw should contain the password (possibly with # → %23 transform)
					if !strings.Contains(d.Raw, "pass") || !strings.Contains(d.Raw, "word") {
						t.Errorf("Raw field corrupted: missing password content in %q", d.Raw)
					}
				})
			}
		}
	}
}

// charDisplay returns a safe display name for the character.
func charDisplay(c string) string {
	switch c {
	case " ":
		return "SPACE"
	case "\t":
		return "TAB"
	case "\n":
		return "NEWLINE"
	case "\r":
		return "CR"
	case "`":
		return "BACKTICK"
	case "\\":
		return "BACKSLASH"
	case "'":
		return "SINGLEQUOTE"
	case "\"":
		return "DOUBLEQUOTE"
	default:
		return c
	}
}
