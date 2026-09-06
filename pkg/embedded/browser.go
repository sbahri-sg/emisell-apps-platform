package embedded

import _ "embed"

// BrowserBridge is the same versioned source used by browser contract tests.
//
//go:embed bridge.mjs
var BrowserBridge string
