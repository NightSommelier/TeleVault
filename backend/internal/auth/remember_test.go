package auth

import "testing"

func TestRememberTokenRoundTrip(t *testing.T) {
	token, err := NewRememberToken()
	if err != nil {
		t.Fatalf("NewRememberToken() error = %v", err)
	}
	raw := token.String()
	parsed, err := ParseRememberToken(raw)
	if err != nil {
		t.Fatalf("ParseRememberToken() error = %v", err)
	}
	if parsed.Selector != token.Selector || parsed.Verifier != token.Verifier {
		t.Fatalf("ParseRememberToken() = %#v, want %#v", parsed, token)
	}
}

func TestParseRememberTokenRejectsInvalid(t *testing.T) {
	for _, input := range []string{"", "abc", "abc.", ".def", "abc:def"} {
		if _, err := ParseRememberToken(input); err == nil {
			t.Fatalf("ParseRememberToken(%q) error = nil, want error", input)
		}
	}
}
