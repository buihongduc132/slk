package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// ---------------------------------------------------------------------
// The seam.
//
// These two shims are the ONLY place this file names the production
// helper, and they are what makes the oracle survive its own fix. Today
// they map onto the two separate helpers. After consolidation there is one
// helper with a mode, and the implementing agent repoints BOTH shims at it
// -- editing these bodies, not the tests above.
//
// That is deliberate: this file is hash-pinned, so the assertions cannot
// be weakened, while the two lines that must legitimately change during
// the refactor are isolated here and clearly labelled.
//
// Whatever the consolidated signature is, the shims must keep returning a
// cmd and must preserve the eager/deferred split the tests pin.
// ---------------------------------------------------------------------

func toastEagerForTest(t *testing.T, a *App, text string, d time.Duration) tea.Cmd {
	t.Helper()
	// CONSOLIDATION: repoint at the single helper's EAGER mode.
	return a.uploadToastCmd(text, d, toastEager)
}

func toastDeferredForTest(t *testing.T, a *App, text string, d time.Duration) tea.Cmd {
	t.Helper()
	// CONSOLIDATION: repoint at the single helper's DEFERRED mode.
	return a.uploadToastCmd(text, d, toastDeferred)
}
