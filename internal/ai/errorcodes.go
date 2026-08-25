package ai

import "strings"

// ErrorCodes returns the condensed error-code quick reference table for a
// profile (hertz | kitex | micro). The table covers the codes an agent most
// often needs when writing error handling. The full code list lives in the
// standalone design doc under docs/ncgo/.
func ErrorCodes(profile string) string {
	type code struct {
		num, name, http, meaning string
	}
	common := []code{
		{"10000", "CodeSystem", "500", "System error"},
		{"10001", "CodeParamInvalid", "400", "Bad request"},
		{"10002", "CodeAuthFailed", "401", "Auth failure"},
		{"10004", "CodeConfigInvalid", "500", "Config error"},
		{"10010", "CodeRPCUnavailable", "502", "RPC unavailable"},
		{"10011", "CodeRPCTimeout", "504", "RPC timeout"},
	}
	var specific []code
	switch profile {
	case "hertz":
		specific = []code{
			{"10108", "CodePermissionDenied", "403", "Permission denied"},
			{"10200", "CodeRateLimited", "429", "Rate limited"},
			{"10202", "CodeReplayRequest", "401", "Replay detected"},
			{"10203", "CodeIdempotencyKeyMissing", "400", "Missing idempotency key"},
			{"10204", "CodeIdempotencyConflict", "409", "Idempotency conflict"},
			{"10304", "CodeCacheUnavailable", "503", "Cache unavailable"},
		}
	case "kitex":
		specific = []code{
			{"10108", "CodePermissionDenied", "403", "Permission denied"},
			{"10200", "CodeRateLimited", "429", "Rate limited"},
			{"10304", "CodeCacheUnavailable", "503", "Cache unavailable"},
		}
	}
	all := append(common, specific...)
	var b strings.Builder
	b.WriteString("| Code | Name | HTTP | Meaning |\n")
	b.WriteString("|------|------|------|---------|\n")
	for _, c := range all {
		b.WriteString("| " + c.num + " | " + c.name + " | " + c.http + " | " + c.meaning + " |\n")
	}
	b.WriteString("| 40100+ | Business codes | 200 | Application-defined |\n")
	return strings.TrimRight(b.String(), "\n")
}
