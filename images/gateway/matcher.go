package gateway

import (
	"strings"
)

// matchRule finds the first matching egress rule for the given hostname.
// Returns the matched rule and whether a match was found.
func matchRule(rules []EgressRule, hostname string) (EgressRule, bool) {
	for _, rule := range rules {
		if ruleMatchesHost(rule, hostname) {
			return rule, true
		}
	}
	return EgressRule{}, false
}

// ruleMatchesHost checks if a rule matches the given hostname.
func ruleMatchesHost(rule EgressRule, hostname string) bool {
	// If rule has host patterns, check them
	if len(rule.Host) > 0 {
		for _, pattern := range rule.Host {
			if hostMatches(pattern, hostname) {
				return true
			}
		}
		return false
	}

	// If rule has no host patterns but has a provider, it matches everything
	// (e.g., docker-api-proxy rules that match by endpoint, not host)
	if rule.Provider != "" && len(rule.Endpoint) == 0 {
		return true
	}

	return false
}

// hostMatches checks if a hostname matches a pattern.
// Supports:
//   - Exact match: "api.github.com"
//   - Wildcard: "*" (matches everything)
//   - Suffix wildcard: "*.github.com" (matches any subdomain)
func hostMatches(pattern, hostname string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == hostname {
		return true
	}
	// Suffix wildcard: *.example.com matches sub.example.com
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(hostname, suffix)
	}
	return false
}
