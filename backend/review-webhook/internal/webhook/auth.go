package webhook

import (
	"crypto/subtle"
	"strings"
)

func authorizationMatches(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1 {
		return true
	}

	for _, prefix := range []string{"Bearer ", "bearer ", "token ", "Token "} {
		if strings.HasPrefix(got, prefix) {
			token := strings.TrimSpace(strings.TrimPrefix(got, prefix))
			if subtle.ConstantTimeCompare([]byte(token), []byte(want)) == 1 {
				return true
			}
		}
	}
	return false
}

func authorizationStatus(got, want string) string {
	if strings.TrimSpace(got) == "" {
		return "missing"
	}
	if authorizationMatches(got, want) {
		return "ok"
	}
	return "mismatch"
}
