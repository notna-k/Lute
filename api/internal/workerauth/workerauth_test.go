package workerauth

import (
	"strings"
	"testing"
)

func TestNewTokenAndSecret(t *testing.T) {
	token, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	secret, err := NewSecret()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, TokenPrefix) || !strings.HasPrefix(secret, SecretPrefix) {
		t.Errorf("prefixes: token %q, secret %q", token, secret)
	}
	// 32 bytes of base32 is 52 characters.
	if got := len(strings.TrimPrefix(token, TokenPrefix)); got != 52 {
		t.Errorf("token body is %d chars, want 52", got)
	}
	again, _ := NewToken()
	if again == token {
		t.Error("two tokens are equal")
	}
	if got := Display(token); got != token[:DisplayLen] {
		t.Errorf("display %q", got)
	}
}

func TestMatches(t *testing.T) {
	secret, _ := NewSecret()
	hash := Hash(secret)
	if !Matches(secret, hash) {
		t.Error("the secret does not match its own hash")
	}
	if Matches(secret+"x", hash) {
		t.Error("a different secret matches")
	}
	if Matches("", "") {
		t.Error("an empty stored hash matches")
	}
}

func TestParseBearer(t *testing.T) {
	tests := []struct {
		header         string
		wantID, wantSc string
		wantErr        bool
	}{
		{header: Bearer("65a1", "lute_ws_abc"), wantID: "65a1", wantSc: "lute_ws_abc"},
		{header: "Bearer 65a1.lute_ws_abc ", wantID: "65a1", wantSc: "lute_ws_abc"},
		{header: "65a1.lute_ws_abc", wantErr: true},
		{header: "Bearer 65a1", wantErr: true},
		{header: "Bearer .secret", wantErr: true},
		{header: "Bearer 65a1.", wantErr: true},
		{header: "", wantErr: true},
	}
	for _, tt := range tests {
		id, secret, err := ParseBearer(tt.header)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseBearer(%q) err = %v, want error %v", tt.header, err, tt.wantErr)
			continue
		}
		if id != tt.wantID || secret != tt.wantSc {
			t.Errorf("ParseBearer(%q) = %q, %q", tt.header, id, secret)
		}
	}
}
