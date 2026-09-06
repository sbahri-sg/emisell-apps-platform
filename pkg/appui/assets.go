// Package appui distributes presentation assets, never authentication behavior.
package appui

import _ "embed"

const Version = "0.1.0"

// CSS is served by the app origin, not injected by the parent dashboard.
//
//go:embed emisell-ui.css
var CSS string
