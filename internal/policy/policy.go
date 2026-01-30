// Package policy provides network allowlist policy evaluation.
package policy

import (
	"strings"

	"github.com/gobwas/glob"
)

// Policy evaluates host allowlist rules.
type Policy struct {
	patterns []glob.Glob
}

// New creates a Policy from host glob patterns.
func New(hosts []string) (*Policy, error) {
	patterns := make([]glob.Glob, 0, len(hosts))
	for _, h := range hosts {
		// Case-insensitive matching per DNS spec
		g, err := glob.Compile(strings.ToLower(h))
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, g)
	}
	return &Policy{patterns: patterns}, nil
}

// Evaluate returns true if the host is allowed.
func (p *Policy) Evaluate(host string) bool {
	// Strip port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	host = strings.ToLower(host)

	for _, pattern := range p.patterns {
		if pattern.Match(host) {
			return true
		}
	}
	return false
}
