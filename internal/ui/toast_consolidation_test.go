package ui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// RED layer for Open Thread 5 / gotcha B2: the status-bar toast exists
// under TWO spellings with DIFFERENT cmd contracts, and picking the wrong
// one is a live defect generator.
//
//	toastWithClear(a, text, d)   reducer_io.go   sets the toast EAGERLY,
//	                                            returns a bare clear tick
//	a.uploadToastCmd(text, dur)  app.go          returns tea.Batch(setter,
//	                                            tick); the toast is NOT
//	                                            applied until the batch runs
//
// That divergence already cost: 6 of the 8 fullscreen failures (the
// suppression path called the batched one while its tests read the status
// bar synchronously), and a 10-minute package hang when someone "fixed" it
// by making uploadToastCmd eager, which broke 14 call sites and left
// firstBatchCmd blocking on a bare tea.Tick.
//
// AGENTS.md's rule for this class is DELETE one, never alias. So the end
// state is ONE implementation. Both SEMANTICS must survive, because both
// are load-bearing:
//   - eager: a caller that returns into a reducer whose test reads the
//     status bar without executing the cmd.
//   - deferred: a caller that composes the result into a larger tea.Batch
//     (app.go:3586 does exactly this), where an eager setter would fire
//     before the runtime ever ran the cmd.
//
// So: one function, an explicit mode, no second body.

// toastHelperNames are the two spellings. Exactly one may survive as a
// declared function in non-test code.
var toastHelperNames = []string{"toastWithClear", "uploadToastCmd"}

// TestToast_OnlyOneHelperImplementationSurvives is the class-level gate:
// it fails while both helpers are still DECLARED. Declaration is the right
// signal, not call count -- a wrapper that merely forwards is still a
// second spelling, which is what "never alias" forbids.
func TestToast_OnlyOneHelperImplementationSurvives(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading internal/ui directory: %v", err)
	}

	found := map[string]string{} // name -> position
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		path := entry.Name()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil {
				continue
			}
			for _, want := range toastHelperNames {
				if fn.Name.Name == want {
					found[want] = path + ":" + fset.Position(fn.Pos()).String()
				}
			}
		}
	}

	if len(found) == 0 {
		t.Fatalf("neither %v is declared any more; the toast helper must still exist", toastHelperNames)
	}
	if len(found) > 1 {
		var b strings.Builder
		for name, pos := range found {
			b.WriteString("\n  " + name + " declared at " + pos)
		}
		t.Errorf("both toast helpers are still declared:%s\n\n"+
			"Consolidate to ONE implementation with an explicit eager/deferred\n"+
			"mode. Do NOT keep one as a thin wrapper around the other -- a\n"+
			"forwarding alias is still two spellings, which is exactly what\n"+
			"AGENTS.md's \"DELETE one, never alias\" rule forbids.", b.String())
	}
}

// TestToast_EagerSemanticsPreserved pins the semantic the SUPPRESSION path
// needs: the toast text is on the status bar straight after the call, with
// the returned cmd NOT executed. reducer_zoom.go's suppression rows in
// fullscreen_red_test.go read statusbarText immediately, so losing this
// breaks them -- and those tests are hash-pinned, so they cannot be
// adjusted to compensate.
func TestToast_EagerSemanticsPreserved(t *testing.T) {
	a := newTestApp(t, withSize(120, 24))
	cmd := toastEagerForTest(t, a, "eager toast text", 2*time.Second)

	if got := statusbarText(a); !strings.Contains(got, "eager toast text") {
		t.Errorf("status bar = %q, want the toast applied WITHOUT running the cmd "+
			"(the suppression path depends on this)", got)
	}
	if cmd == nil {
		t.Error("cmd = nil, want the clear tick so the toast does not persist forever")
	}
}

// TestToast_DeferredSemanticsPreserved pins the other semantic: nothing
// touches the status bar until the returned cmd runs. app.go composes this
// form INSIDE a larger tea.Batch, where an eager setter would fire at
// construction time -- before the runtime executed anything.
func TestToast_DeferredSemanticsPreserved(t *testing.T) {
	a := newTestApp(t, withSize(120, 24))
	cmd := toastDeferredForTest(t, a, "deferred toast text", 2*time.Second)

	if got := statusbarText(a); strings.Contains(got, "deferred toast text") {
		t.Errorf("status bar = %q, want it EMPTY until the cmd runs "+
			"(app.go composes this form inside a tea.Batch)", got)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want a batch of (setter, tick)")
	}
	// Run only the setter; the tick would sleep. Bounded so a contract
	// break fails this row instead of hanging the package (see B3).
	msg, ok := cmdMsgWithin(t, cmd, time.Second)
	if !ok {
		t.Fatal("cmd() did not return within 1s: the deferred form must be a " +
			"tea.Batch, not a bare tea.Tick")
	}
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want tea.BatchMsg", msg)
	}
	for _, c := range batch {
		if c == nil {
			continue
		}
		if _, ran := cmdMsgWithin(t, c, 200*time.Millisecond); ran {
			break // the setter returns immediately; the tick does not
		}
	}
	if got := statusbarText(a); !strings.Contains(got, "deferred toast text") {
		t.Errorf("status bar = %q, want the toast applied after the batch's setter ran", got)
	}
}
