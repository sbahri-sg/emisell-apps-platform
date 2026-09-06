package main

import (
	"strings"
	"testing"

	"emisell.app/platform/pkg/appui"
)

func TestSharedUIKitAndDemoMarkup(t *testing.T) {
	d, err := newDemo()
	if err != nil {
		t.Fatal(err)
	}
	_, frame := d.handlers()
	w := request(frame, "GET", appOrigin+"/emisell-ui.css", "", nil)
	if w.Code != 200 || w.Body.String() != appui.CSS || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/css") {
		t.Fatal("shared UI asset not served")
	}
	for _, path := range []string{"/", "/seller"} {
		page := request(frame, "GET", appOrigin+path, "", nil)
		for _, expected := range []string{`href="/emisell-ui.css"`, `class="eui"`, `id="refresh" type="button"`, `id="merchant"`, `id="actor"`, `id="expiry"`, `aria-live="polite"`} {
			if !strings.Contains(page.Body.String(), expected) {
				t.Fatalf("%s missing %s", path, expected)
			}
		}
		if strings.Contains(page.Body.String(), `href="/demo.css"`) {
			t.Fatal("app still uses host styles")
		}
	}
	for _, rule := range []string{":focus-visible", ":disabled", "@media (max-width: 600px)", "overflow-wrap: anywhere"} {
		if !strings.Contains(appui.CSS, rule) {
			t.Fatalf("missing UI state: %s", rule)
		}
	}
}
