package publicapi

import (
	"net/http"
	"testing"

	"github.com/lute/api/internal/db/models"
)

type keyList struct {
	Keys []keySummary `json:"api_keys"`
}

func (f *fixture) listKeys(u *models.User, scope string) []keySummary {
	f.t.Helper()
	path := "/api/v1/api-keys"
	if scope != "" {
		path += "?scope=" + scope
	}
	rec := f.panel(u, http.MethodGet, path, nil)
	if rec.Code != http.StatusOK {
		f.t.Fatalf("list %s keys: %d %s", scope, rec.Code, rec.Body)
	}
	return decode[keyList](f.t, rec).Keys
}

func hasKey(keys []keySummary, id string) bool {
	for _, k := range keys {
		if k.ID == id {
			return true
		}
	}
	return false
}

func TestAccountKeys(t *testing.T) {
	f := newFixture(t)
	anton, bob := f.user("anton@acme.dev"), f.user("bob@acme.dev")

	t.Run("Success - a key is an account key unless asked otherwise", func(t *testing.T) {
		k := f.key(anton, "laptop", "")
		if k.Scope != "account" {
			t.Errorf("scope = %q, want account", k.Scope)
		}
	})

	t.Run("Success - only its owner sees an account key", func(t *testing.T) {
		k := f.key(anton, "scripts", "account")
		if got := f.listKeys(anton, ""); !hasKey(got, k.ID) {
			t.Errorf("anton's keys %+v miss his own key", got)
		}
		if got := f.listKeys(bob, "account"); hasKey(got, k.ID) {
			t.Errorf("bob sees anton's account key: %+v", got)
		}
		if got := f.listKeys(bob, "service"); hasKey(got, k.ID) {
			t.Errorf("anton's account key is listed as a service key: %+v", got)
		}
	})

	t.Run("Fail - nobody but its owner revokes it", func(t *testing.T) {
		k := f.key(anton, "precious", "account")
		wantError(t, f.panel(bob, http.MethodDelete, "/api/v1/api-keys/"+k.ID, nil), http.StatusNotFound, "not_found")
		if rec := f.public(k.Token, http.MethodGet, "/whoami", nil); rec.Code != http.StatusOK {
			t.Errorf("bob's revoke attempt broke anton's key: %d", rec.Code)
		}
	})

	t.Run("Fail - it stops working when its owner is removed", func(t *testing.T) {
		carol := f.user("carol@acme.dev")
		k := f.key(carol, "laptop", "account")
		if rec := f.public(k.Token, http.MethodGet, "/whoami", nil); rec.Code != http.StatusOK {
			t.Fatalf("the key does not work to begin with: %d", rec.Code)
		}
		deleteUser(t, f, carol)
		wantError(t, f.public(k.Token, http.MethodGet, "/whoami", nil), http.StatusUnauthorized, "unauthorized")
	})

	t.Run("Fail - an unknown scope is rejected and names the field", func(t *testing.T) {
		rec := f.panel(anton, http.MethodPost, "/api/v1/api-keys", map[string]string{"name": "x", "scope": "admin"})
		e := wantError(t, rec, http.StatusBadRequest, "validation_failed")
		if e.Error.Fields["scope"] == "" {
			t.Errorf("fields = %v, want scope", e.Error.Fields)
		}
		wantError(t, f.panel(anton, http.MethodGet, "/api/v1/api-keys?scope=admin", nil), http.StatusBadRequest, "validation_failed")
	})
}

func TestServiceKeys(t *testing.T) {
	f := newFixture(t)
	anton, bob := f.user("anton@acme.dev"), f.user("bob@acme.dev")

	t.Run("Success - every user sees a service key and who created it", func(t *testing.T) {
		k := f.key(anton, "ci", "service")
		if k.Scope != "service" {
			t.Errorf("scope = %q, want service", k.Scope)
		}
		got := f.listKeys(bob, "service")
		if len(got) != 1 || got[0].ID != k.ID {
			t.Fatalf("bob's service keys = %+v, want anton's ci key", got)
		}
		if got[0].CreatorEmail != "anton@acme.dev" || got[0].CreatedBy != anton.ID.Hex() || got[0].Scope != "service" {
			t.Errorf("key = %+v, want created by anton", got[0])
		}
		if hasKey(f.listKeys(anton, "account"), k.ID) {
			t.Error("the service key is also listed as anton's account key")
		}
	})

	t.Run("Success - any user revokes it", func(t *testing.T) {
		k := f.key(anton, "cron", "service")
		if rec := f.panel(bob, http.MethodDelete, "/api/v1/api-keys/"+k.ID, nil); rec.Code != http.StatusNoContent {
			t.Fatalf("bob revokes a service key: %d %s", rec.Code, rec.Body)
		}
		wantError(t, f.public(k.Token, http.MethodGet, "/whoami", nil), http.StatusUnauthorized, "unauthorized")
	})

	t.Run("Success - it keeps working when its creator is removed", func(t *testing.T) {
		dave := f.user("dave@acme.dev")
		k := f.key(dave, "release-bot", "service")
		deleteUser(t, f, dave)
		if rec := f.public(k.Token, http.MethodGet, "/whoami", nil); rec.Code != http.StatusOK {
			t.Errorf("the service key died with its creator: %d %s", rec.Code, rec.Body)
		}
	})

	t.Run("Fail - worker routes need an account key", func(t *testing.T) {
		service := f.key(anton, "ci-workers", "service")
		wantError(t, f.public(service.Token, http.MethodGet, "/workers", nil), http.StatusForbidden, "forbidden")

		account := f.key(anton, "laptop", "account")
		if rec := f.public(account.Token, http.MethodGet, "/workers", nil); rec.Code != http.StatusOK {
			t.Errorf("an account key lists workers: %d %s", rec.Code, rec.Body)
		}
	})
}

// deleteUser removes u's row, as a future "remove account" would; nothing else is touched.
func deleteUser(t *testing.T, f *fixture, u *models.User) {
	t.Helper()
	if err := f.db.Delete(&models.User{}, "id = ?", u.ID.Hex()).Error; err != nil {
		t.Fatalf("delete user: %v", err)
	}
}
