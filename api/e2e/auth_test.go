//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/lute/api/e2e/harness"
)

// TestSignInFlow covers getting into the panel and staying in: the seeded admin, the
// refresh rotation a long session depends on, and the two ways a session ends.
func TestSignInFlow(t *testing.T) {
	stack := newBareStack(t)

	t.Run("Success - seeded admin signs in; token identifies the account", func(t *testing.T) {
		c := stack.Client()

		session, err := c.Login(harness.AdminEmail, harness.AdminPassword)
		if err != nil {
			t.Fatalf("login: %v", err)
		}
		if session.AccessToken == "" {
			t.Fatal("no access token issued")
		}
		if session.TokenType != "Bearer" {
			t.Errorf("token_type = %q, want Bearer", session.TokenType)
		}

		me, err := c.Me()
		if err != nil {
			t.Fatalf("me: %v", err)
		}
		want := harness.User{ID: session.User.ID, Email: harness.AdminEmail, DisplayName: "Admin"}
		if diff := cmp.Diff(want, me); diff != "" {
			t.Errorf("signed-in user mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("Fail - wrong password; no token and no session", func(t *testing.T) {
		c := stack.Client()

		if _, err := c.Login(harness.AdminEmail, "not-the-password"); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Fatalf("login with a wrong password: err = %v, want 401", err)
		}
		// A failed login must not leave a usable refresh cookie behind.
		if _, err := c.Refresh(); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Fatalf("refresh after failed login: err = %v, want 401", err)
		}
	})

	t.Run("Fail - unknown account; same answer as a wrong password", func(t *testing.T) {
		c := stack.Client()
		if _, err := c.Login("nobody@e2e.test", harness.AdminPassword); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Fatalf("login as an unknown user: err = %v, want 401", err)
		}
	})

	t.Run("Fail - no token; the panel's endpoints stay shut", func(t *testing.T) {
		c := stack.Client()

		if _, err := c.Me(); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Errorf("GET /auth/me anonymously: err = %v, want 401", err)
		}
		if _, err := c.ListJobDefs(); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Errorf("GET /job-definitions anonymously: err = %v, want 401", err)
		}
		if _, err := c.ListWorkers(); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Errorf("GET /workers anonymously: err = %v, want 401", err)
		}
		if _, err := c.CreateClaimCode(); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Errorf("POST /workers/claim-code anonymously: err = %v, want 401", err)
		}
	})

	t.Run("Success - refresh rotates the token and retires the old one", func(t *testing.T) {
		c := stack.AdminClient()

		first := c.RefreshCookie()
		if first == "" {
			t.Fatal("login set no refresh cookie")
		}

		refreshed, err := c.Refresh()
		if err != nil {
			t.Fatalf("refresh: %v", err)
		}
		if refreshed.AccessToken == "" {
			t.Fatal("refresh issued no access token")
		}
		second := c.RefreshCookie()
		if second == first {
			t.Fatal("refresh returned the same refresh token; a stolen one would live forever")
		}

		// Replaying a rotated token is how a stolen cookie shows up. It must fail,
		// and it must take the whole family with it.
		c.SetRefreshCookie(first)
		if _, err := c.Refresh(); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Fatalf("replaying a rotated refresh token: err = %v, want 401", err)
		}
		c.SetRefreshCookie(second)
		if _, err := c.Refresh(); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Fatalf("refresh after a detected replay: err = %v, want 401 (the family should be revoked)", err)
		}
	})

	t.Run("Success - logout ends the session for good", func(t *testing.T) {
		c := stack.AdminClient()

		if err := c.Logout(); err != nil {
			t.Fatalf("logout: %v", err)
		}
		if _, err := c.Refresh(); harness.StatusOf(err) != http.StatusUnauthorized {
			t.Fatalf("refresh after logout: err = %v, want 401", err)
		}
	})
}
