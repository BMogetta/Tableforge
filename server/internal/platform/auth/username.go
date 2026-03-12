package auth

import "strings"

// sanitizeUsername lowercases and strips characters that aren't alphanumeric or
// underscores, so GitHub logins map cleanly to our username format.
func SanitizeUsername(login string) string {
	login = strings.ToLower(login)
	var b strings.Builder
	for _, r := range login {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else if r == '-' {
			b.WriteRune('_')
		}
	}
	return b.String()
}
