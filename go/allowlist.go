package wgrok

import (
	"log"
	"strings"
)

// patternType represents the classification of an allowlist pattern.
type patternType int

const (
	exact patternType = iota
	wildcardPrefix
	bareDomain
)

// pattern represents a normalized allowlist pattern with its type.
type pattern struct {
	pType patternType
	value string
}

// Allowlist validates email addresses against a list of allowed patterns.
//
// Supported formats:
//   - "domain.tld"      → matches *@domain.tld (bare domain)
//   - "*@domain.tld"    → wildcard prefix match
//   - "user@domain.tld" → exact match (case-insensitive)
//
// Patterns with [, ], or ? are rejected (dangerous).
type Allowlist struct {
	patterns []pattern
}

// NewAllowlist creates an Allowlist from the given patterns.
func NewAllowlist(patterns []string) *Allowlist {
	var normalized []pattern
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Reject patterns with dangerous characters
		if strings.ContainsAny(p, "[]?") {
			log.Printf("Rejecting dangerous allowlist pattern: %s", p)
			continue
		}
		pLower := strings.ToLower(p)
		if strings.HasPrefix(pLower, "*@") {
			// Wildcard prefix: *@domain.tld
			domain := pLower[2:]
			normalized = append(normalized, pattern{pType: wildcardPrefix, value: domain})
		} else if strings.Contains(pLower, "@") {
			// Exact match: user@domain.tld
			normalized = append(normalized, pattern{pType: exact, value: pLower})
		} else {
			// Bare domain: domain.tld
			normalized = append(normalized, pattern{pType: bareDomain, value: pLower})
		}
	}
	return &Allowlist{patterns: normalized}
}

// IsAllowed checks if an email address matches any allowed pattern (case-insensitive).
func (a *Allowlist) IsAllowed(email string) bool {
	emailLower := strings.ToLower(email)
	for _, p := range a.patterns {
		switch p.pType {
		case exact:
			if emailLower == p.value {
				return true
			}
		case wildcardPrefix:
			// Check if email is at the domain
			if strings.HasSuffix(emailLower, "@"+p.value) {
				return true
			}
		case bareDomain:
			// Check if email's domain matches
			if strings.HasSuffix(emailLower, "@"+p.value) {
				return true
			}
		}
	}
	return false
}
