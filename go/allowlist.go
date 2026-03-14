package wgrok

import (
	"path/filepath"
	"strings"
)

// Allowlist validates email addresses against a list of allowed patterns.
//
// Supported formats:
//   - "domain.tld"      → matches *@domain.tld
//   - "*@domain.tld"    → wildcard match
//   - "user@domain.tld" → exact match
type Allowlist struct {
	patterns []string
}

// NewAllowlist creates an Allowlist from the given patterns.
func NewAllowlist(patterns []string) *Allowlist {
	var normalized []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.Contains(p, "@") {
			normalized = append(normalized, "*@"+p)
		} else {
			normalized = append(normalized, p)
		}
	}
	return &Allowlist{patterns: normalized}
}

// IsAllowed checks if an email address matches any allowed pattern (case-insensitive).
func (a *Allowlist) IsAllowed(email string) bool {
	emailLower := strings.ToLower(email)
	for _, pattern := range a.patterns {
		matched, err := filepath.Match(strings.ToLower(pattern), emailLower)
		if err == nil && matched {
			return true
		}
	}
	return false
}
