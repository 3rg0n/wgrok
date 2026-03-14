package wgrok

import (
	"encoding/json"
	"os"
	"testing"
)

type allowlistCases struct {
	Cases []struct {
		Name     string   `json:"name"`
		Patterns []string `json:"patterns"`
		Email    string   `json:"email"`
		Expected bool     `json:"expected"`
	} `json:"cases"`
}

func loadAllowlistCases(t *testing.T) allowlistCases {
	t.Helper()
	data, err := os.ReadFile("../tests/allowlist_cases.json")
	if err != nil {
		t.Fatalf("load allowlist cases: %v", err)
	}
	var cases allowlistCases
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse allowlist cases: %v", err)
	}
	return cases
}

func TestAllowlist(t *testing.T) {
	cases := loadAllowlistCases(t)
	for _, tc := range cases.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			al := NewAllowlist(tc.Patterns)
			got := al.IsAllowed(tc.Email)
			if got != tc.Expected {
				t.Errorf("IsAllowed(%q) with patterns %v = %v, want %v", tc.Email, tc.Patterns, got, tc.Expected)
			}
		})
	}
}
