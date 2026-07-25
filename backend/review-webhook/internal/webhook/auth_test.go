package webhook

import "testing"

func TestAuthorizationMatches(t *testing.T) {
	want := "12345"
	for _, got := range []string{"12345", "Bearer 12345", "token 12345"} {
		if !authorizationMatches(got, want) {
			t.Fatalf("expected %q to match", got)
		}
	}
	if authorizationMatches("", want) {
		t.Fatal("empty should not match")
	}
	if authorizationMatches("wrong", want) {
		t.Fatal("wrong token should not match")
	}
}

func TestAuthorizationStatus(t *testing.T) {
	if authorizationStatus("", "12345") != "missing" {
		t.Fatal("expected missing")
	}
	if authorizationStatus("12345", "12345") != "ok" {
		t.Fatal("expected ok")
	}
	if authorizationStatus("nope", "12345") != "mismatch" {
		t.Fatal("expected mismatch")
	}
}
