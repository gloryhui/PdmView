//go:build e2e

package main

// Build with -tags e2e and start with MYGO_AUTOMATION=<socket> to expose
// MyGo's official newline-delimited JSON automation protocol.
import _ "github.com/egoist/mygo/plugins/automation"
