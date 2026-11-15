// SPDX-License-Identifier: MIT
package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestTheDocumentedExitCodeWins: usage errors (2) come first, then what the
// adapter does not support (5), then the fixture (4), whatever the order of
// the flags (docs/CONTAINER_CONTRACT.md, Commands). No case reaches the
// network.
func TestTheDocumentedExitCodeWins(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.json")
	const url = "ws://127.0.0.1:1"
	for _, c := range []struct {
		want int
		args []string
	}{
		{2, []string{"server", "--version", "1.6", "--bind-address", "notanip"}},
		{4, []string{"server", "--version", "1.6", "--fixture", missing}},
		// A station does not send Reset: not supported, whatever the fixture.
		{5, []string{"client", "Reset", "--version", "1.6", "--url", url, "--fixture", missing}},
		{4, []string{"client", "Heartbeat", "--version", "1.6", "--url", url, "--fixture", missing}},
	} {
		if got := run(c.args); got != c.want {
			t.Errorf("%q: exit %d, want %d", strings.Join(c.args, " "), got, c.want)
		}
	}
}
