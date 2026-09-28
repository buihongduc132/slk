package ui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestToast_OnlyOneConstructorByShape closes the gap gotcha B2's audit entry
// pointed at without quite describing.
//
// internal/ui/toast_consolidation_test.go already asserts that only one toast
// helper survives, and it works — an injected duplicate `toastWithClear` fails
// it. But it matches a hardcoded name list:
//
//	var toastHelperNames = []string{"toastWithClear", "uploadToastCmd"}
//
// so novelty of name defeats it. Verified at 5665841: a third constructor named
// `flashToast`, wired and compiling, left that oracle GREEN and `go test ./...`
// GREEN. The consolidation rule was enforced against two spellings rather than
// against the shape it actually cares about.
//
// This file is deliberately SEPARATE from that oracle, which is hash-pinned.
// The pin rule's sanctioned move is to add a new unpinned file alongside, never
// to edit the pinned one — so this adds an assertion rather than changing one.
// The two are complementary: the name oracle catches a revival of either
// historical spelling, this catches any future constructor whatever it is called.
//
// THE SHAPE, and why it is drawn this narrowly. Three functions in this package
// both return tea.Cmd and call SetToast, and only one is a constructor:
//
//	uploadToastCmd(text string, dur time.Duration, mode toastMode) tea.Cmd
//	handlePresenceCustomSnoozeMode(a *App, msg tea.KeyMsg) tea.Cmd
//	(a reducer in reducer_io.go, which sets a progress toast inline)
//
// The latter two set a toast incidentally while doing something else; their
// parameters are a key message and app state. A constructor's parameters ARE the
// toast: its text and its lifetime. So the rule is "returns tea.Cmd, calls
// SetToast, AND takes both a string and a time.Duration" — which selects the
// constructor and leaves ordinary call sites alone. A guard that fired on every
// function that happens to raise a toast would be deleted by the first person it
// inconvenienced, and rightly.
func TestToast_OnlyOneConstructorByShape(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package directory: %v", err)
	}

	fset := token.NewFileSet()
	var scanned int
	var found []string

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++

		file, parseErr := parser.ParseFile(fset, name, nil, 0)
		if parseErr != nil {
			t.Fatalf("parsing %s: %v", name, parseErr)
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Type == nil || fn.Body == nil {
				continue
			}
			if !returnsTeaCmd(fn.Type) || !takesTextAndDuration(fn.Type) || !callsSetToast(fn.Body) {
				continue
			}
			found = append(found, fn.Name.Name+" ("+name+":"+
				strconv.Itoa(fset.Position(fn.Pos()).Line)+")")
		}
	}

	// V2/V3: an enumeration that read nothing would make this vacuously green.
	if scanned == 0 {
		t.Fatal("scanned 0 non-test .go files -- the walk is broken, so a green " +
			"result here carries no information")
	}

	switch len(found) {
	case 1:
		t.Logf("scanned %d file(s); one toast-cmd constructor by shape: %s", scanned, found[0])
	case 0:
		t.Errorf("scanned %d file(s) and found NO function matching the toast-cmd "+
			"constructor shape (returns tea.Cmd, takes a string and a "+
			"time.Duration, calls SetToast).\n\n"+
			"Either the constructor was renamed out of this shape -- in which case "+
			"update this test's rule deliberately -- or the walk has stopped seeing "+
			"it, in which case this guard is dead and would not notice a duplicate.",
			scanned)
	default:
		t.Errorf("%d functions match the toast-cmd constructor shape:\n  %s\n\n"+
			"Consolidate to ONE constructor with an explicit eager/deferred mode.\n"+
			"Do NOT keep one as a thin wrapper around the other -- a forwarding\n"+
			"alias is still two constructors, which is what AGENTS.md's \"DELETE\n"+
			"one, never alias\" rule forbids.\n\n"+
			"Note this test is shape-based on purpose: toast_consolidation_test.go\n"+
			"matches a fixed name list, so a constructor under a new name passes it.\n"+
			"If you are adding a deliberate second constructor, changing THIS rule is\n"+
			"the decision you are making -- say why in the commit.",
			len(found), strings.Join(found, "\n  "))
	}
}

// returnsTeaCmd reports whether the function's last result is tea.Cmd.
func returnsTeaCmd(ft *ast.FuncType) bool {
	if ft.Results == nil || len(ft.Results.List) == 0 {
		return false
	}
	last := ft.Results.List[len(ft.Results.List)-1]
	sel, ok := last.Type.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != "Cmd" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "tea"
}

// takesTextAndDuration reports whether the parameters include both a plain
// string and a time.Duration. That pair is what makes a function a toast
// CONSTRUCTOR rather than a call site that happens to raise one: the text is
// the toast's content and the duration is its lifetime.
func takesTextAndDuration(ft *ast.FuncType) bool {
	if ft.Params == nil {
		return false
	}
	var hasString, hasDuration bool
	for _, p := range ft.Params.List {
		switch typ := p.Type.(type) {
		case *ast.Ident:
			if typ.Name == "string" {
				hasString = true
			}
		case *ast.SelectorExpr:
			if typ.Sel != nil && typ.Sel.Name == "Duration" {
				if pkg, ok := typ.X.(*ast.Ident); ok && pkg.Name == "time" {
					hasDuration = true
				}
			}
		}
	}
	return hasString && hasDuration
}

// callsSetToast reports whether the body calls anything named SetToast, on any
// receiver. Receiver-agnostic on purpose: a second constructor reaching the
// status bar through a different field or a wrapper is still a second
// constructor, and pinning the receiver would let that through.
func callsSetToast(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel != nil &&
			sel.Sel.Name == "SetToast" {
			found = true
			return false
		}
		return true
	})
	return found
}
