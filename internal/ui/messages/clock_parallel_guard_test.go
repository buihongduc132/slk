package messages

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestNowFunc_NoParallelTestTouchesTheClock replaces a comment with a check.
//
// nowFunc (model.go) is a package-level variable with no mutex. The doc comment
// there states the invariant that makes that safe, ending: "no test in the repo
// calls t.Parallel. A future parallel test, or any production caller, would need
// this to become an atomic.Value or a struct field."
//
// That was prose asking future authors to maintain an invariant by hand, which
// AGENTS.md's Conventions section explicitly calls out: "when you find a comment
// standing in for a check, replace it with the check." This is the check.
//
// WHAT IT DOES NOT DO: it does not ban t.Parallel() repo-wide. t.Parallel() is
// ordinarily good practice, and forbidding it everywhere to protect one global
// would trade a real capability for a cheap guarantee. The actual invariant is
// narrower — no file that drives this clock may also run its tests in parallel —
// so that is exactly what is asserted. Tests elsewhere stay free to parallelise.
//
// SCOPE: every _test.go file in the module that mentions SetNowFunc. That set is
// discovered rather than hardcoded, because SetNowFunc is exported and already
// has callers in three separate test binaries (this package, internal/ui, and
// internal/ui/thread); a hardcoded list would go stale the first time a fourth
// package injects a clock.
//
// If this test fails, the honest fix is usually NOT to remove the t.Parallel().
// It is to make nowFunc safe — an atomic.Pointer[func() time.Time], or a struct
// field on Model — and then delete this guard along with the comment it enforces.
func TestNowFunc_NoParallelTestTouchesTheClock(t *testing.T) {
	root := moduleRoot(t)

	var scanned, offenders int
	var details []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "testdata" {
				return fs.SkipDir
			}
			// Skip any nested module. A git worktree, a vendored copy or a
			// nested example carries its own go.mod and is NOT this module's
			// source. This repo keeps agent worktrees under .claude/worktrees/,
			// each a full tree copy holding its own copy of these very files,
			// so without this the walk reads thousands of foreign files and
			// reports duplicates of every finding. Same remedy as the xdg
			// single-source oracle, for the same reason.
			if path != root {
				if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
					return fs.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if !strings.Contains(string(src), "SetNowFunc") {
			return nil
		}

		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, src, 0)
		if parseErr != nil {
			return parseErr
		}

		// Only the PACKAGE-LEVEL clock is unguarded. sidebar.Model carries its
		// own SetNowFunc backed by a struct field (internal/ui/sidebar/model.go),
		// and a parallel test driving that is perfectly safe — each Model has its
		// own. So a plain string match is too coarse: it flags
		// sidebar/staleness_test.go's `m.SetNowFunc(...)` and would fail a test
		// that is not in fact at risk. A guard with false positives gets deleted
		// by the next person it inconveniences, so the AST decides which
		// SetNowFunc this is.
		if !drivesPackageClock(file) {
			return nil
		}
		scanned++

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "Parallel" {
				return true
			}
			offenders++
			details = append(details, "\n  "+rel+":"+
				strconv.Itoa(fset.Position(call.Pos()).Line)+" calls t.Parallel()")
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	// V2/V3: an enumeration that matched nothing would make this vacuously
	// green. SetNowFunc has callers in three packages; if this hits zero the
	// walk is broken, not the repo clean.
	if scanned == 0 {
		t.Fatalf("scanned 0 test files mentioning SetNowFunc under %s -- "+
			"the walk is broken, so a green result here would mean nothing", root)
	}

	if offenders > 0 {
		t.Errorf("%d test file(s) drive nowFunc via SetNowFunc AND call t.Parallel():%s\n\n"+
			"nowFunc (model.go) has no mutex. Its safety rests on every write coming\n"+
			"from test setup on the test goroutine before any render. A parallel test\n"+
			"breaks that.\n\n"+
			"Preferred fix: make nowFunc an atomic.Pointer[func() time.Time] or a\n"+
			"Model field, then delete this guard and the comment it enforces. Removing\n"+
			"the t.Parallel() call is the lesser fix -- it keeps the limitation.",
			offenders, strings.Join(details, ""))
	}

	t.Logf("%d test file(s) drive the clock; none calls t.Parallel()", scanned)
}

// drivesPackageClock reports whether file calls the PACKAGE-LEVEL SetNowFunc
// that writes nowFunc, as opposed to a method of the same name on some value.
//
// Two spellings qualify:
//
//	SetNowFunc(fn)           // from inside package messages
//	messages.SetNowFunc(fn)  // from another package
//
// Anything else with that selector name is a method on a receiver — chiefly
// sidebar.Model's own injectable clock, which is a struct field and therefore
// per-instance and parallel-safe. `m.SetNowFunc(...)` and
// `a.sidebar.SetNowFunc(...)` are both correctly rejected: the first has an
// identifier receiver that is not the package name, the second a nested
// selector.
func drivesPackageClock(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			// Unqualified: only reachable from inside this package.
			if fn.Name == "SetNowFunc" {
				found = true
			}
		case *ast.SelectorExpr:
			if fn.Sel == nil || fn.Sel.Name != "SetNowFunc" {
				return true
			}
			// Qualified by the package name is the global; a receiver
			// expression of any other shape is a method.
			if pkg, ok := fn.X.(*ast.Ident); ok && pkg.Name == "messages" {
				found = true
			}
		}
		return !found
	})
	return found
}

// moduleRoot walks up from the working directory to the directory holding
// go.mod. Tests run with their package directory as cwd, so this package's
// tests start three levels down.
func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod found walking up from the test working directory")
		}
		dir = parent
	}
}
