package localhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRemoteEgressIsPinnedAndDoesNotRedirect(t *testing.T) {
	for _, origin := range []string{"https://example.com", "http://localhost:8091", "http://127.0.0.1:8091/path", "http://user@127.0.0.1:8091", "http://169.254.169.254:80"} {
		if _, err := Client(origin); err == nil {
			t.Fatal("unsafe origin accepted")
		}
	}
	called := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer source.Close()
	c, err := Client(source.URL)
	if err != nil {
		t.Fatal(err)
	}
	response, err := c.Get(source.URL)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 302 || called {
		t.Fatal("redirect followed")
	}
	if _, err = c.Get(target.URL); err == nil || called {
		t.Fatal("origin restriction bypassed")
	}
}
