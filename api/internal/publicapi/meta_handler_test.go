package publicapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/lute/api/internal/version"
)

func TestVersion(t *testing.T) {
	f := newFixture(t)

	t.Run("Success - answers without a key", func(t *testing.T) {
		rec := f.public("", http.MethodGet, "/version", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
		}
		got := decode[VersionResponse](t, rec)
		if got.Version != version.Core {
			t.Errorf("version = %q, want %q", got.Version, version.Core)
		}
		if got.APILevel != APILevel || APILevel < 1 {
			t.Errorf("api_level = %d, want %d (and at least 1)", got.APILevel, APILevel)
		}
	})

	t.Run("Success - a bad key does not get in the way", func(t *testing.T) {
		if rec := f.public("lute_sk_nonsense", http.MethodGet, "/version", nil); rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200: %s", rec.Code, rec.Body)
		}
	})
}

func TestWhoAmI(t *testing.T) {
	f := newFixture(t)
	anton := f.user("anton@acme.dev")
	account := f.key(anton, "laptop", "account")
	service := f.key(anton, "ci", "service")

	t.Run("Success - an account key names itself and the user it acts as", func(t *testing.T) {
		rec := f.public(account.Token, http.MethodGet, "/whoami", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
		}
		got := decode[WhoAmIResponse](t, rec)
		want := KeyInfo{ID: account.ID, Name: "laptop", Prefix: account.Prefix, Scope: "account"}
		if got.Key != want {
			t.Errorf("key = %+v, want %+v", got.Key, want)
		}
		if got.User == nil {
			t.Fatal("user is missing; an account key acts as its owner")
		}
		if got.User.ID != anton.ID.Hex() || got.User.Email != "anton@acme.dev" {
			t.Errorf("user = %+v, want anton", got.User)
		}
	})

	t.Run("Success - a service key acts as itself, with no user", func(t *testing.T) {
		rec := f.public(service.Token, http.MethodGet, "/whoami", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body)
		}
		got := decode[WhoAmIResponse](t, rec)
		if got.Key.Scope != "service" || got.Key.Name != "ci" || got.Key.ID != service.ID {
			t.Errorf("key = %+v, want the service key ci", got.Key)
		}
		if got.User != nil {
			t.Errorf("user = %+v, want none for a service key", got.User)
		}
		if strings.Contains(rec.Body.String(), `"user"`) {
			t.Errorf("the response has a user field at all: %s", rec.Body)
		}
	})

	t.Run("Success - the key itself is never echoed", func(t *testing.T) {
		for _, token := range []string{account.Token, service.Token} {
			rec := f.public(token, http.MethodGet, "/whoami", nil)
			if strings.Contains(rec.Body.String(), token) {
				t.Errorf("whoami printed the key: %s", rec.Body)
			}
		}
	})

	t.Run("Fail - no key, a forged key and a revoked key are refused", func(t *testing.T) {
		wantError(t, f.public("", http.MethodGet, "/whoami", nil), http.StatusUnauthorized, "unauthorized")
		wantError(t, f.public("lute_sk_aaaaaaaaaaaaaaaaaaaaaaaa", http.MethodGet, "/whoami", nil), http.StatusUnauthorized, "unauthorized")

		doomed := f.key(anton, "doomed", "account")
		if rec := f.panel(anton, http.MethodDelete, "/api/v1/api-keys/"+doomed.ID, nil); rec.Code != http.StatusNoContent {
			t.Fatalf("revoke: %d %s", rec.Code, rec.Body)
		}
		wantError(t, f.public(doomed.Token, http.MethodGet, "/whoami", nil), http.StatusUnauthorized, "unauthorized")
	})
}
