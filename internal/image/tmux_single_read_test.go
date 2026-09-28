package image

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The B35 class: one environment variable, one package, two reads, only one of
// them injectable. capability.go declares `var getenv = os.Getenv` expressly so
// tests can inject values, and kitty.go's inTmux was reading os.Getenv("TMUX")
// directly — so a test faking getenv saw the fake in capability.go and the real
// process environment in kitty.go, and two image-capability decisions inside one
// package could disagree with each other.
//
// kitty.go:72 carries a comment explaining that fix. Nothing enforced it. I
// verified that by mutation at 3061399: swapping inTmux's getenv("TMUX") back to
// os.Getenv("TMUX") (adding the os import so the tree still builds) left
// `go test ./...` fully GREEN. The comment was the only thing holding the
// invariant, which AGENTS.md's Conventions section names as the anti-pattern to
// replace with a check.
//
// Two tests below, deliberately. The first catches exactly the regression that
// was fixed; the second catches the whole class, including a new raw read at a
// call site nobody has written yet.

// TestInTmux_ReadsTheInjectableAccessor is the behavioural half. It fails if
// inTmux stops routing through the package's getenv variable.
func TestInTmux_ReadsTheInjectableAccessor(t *testing.T) {
	saved := getenv
	t.Cleanup(func() { getenv = saved })

	// Deliberately NOT os.Setenv/t.Setenv: reading the real environment is the
	// defect. A fake that the real environment cannot satisfy is what makes this
	// test able to fail — if inTmux calls os.Getenv, it sees the ambient value
	// (almost always empty) and never this sentinel.
	var asked []string
	getenv = func(k string) string {
		asked = append(asked, k)
		if k == "TMUX" {
			return "/tmp/tmux-1000/default,12345,0"
		}
		return ""
	}

	if !inTmux() {
		t.Errorf("inTmux() = false with getenv injecting a non-empty TMUX; " +
			"it is reading the process environment instead of the package's " +
			"injectable accessor (B35). Route it through getenv.")
	}
	if len(asked) == 0 {
		t.Error("inTmux() never called the injected getenv at all, so it cannot " +
			"be reading TMUX through the accessor")
	}

	// The negative leg: with the accessor reporting empty, inTmux must be false
	// even when the real environment has TMUX set. Without this, a hardcoded
	// `return true` would satisfy the case above.
	getenv = func(string) string { return "" }
	if inTmux() {
		t.Error("inTmux() = true with getenv returning empty; it is not honouring " +
			"the accessor's answer")
	}
}

// TestImage_TMUXHasExactlyOneRawEnvRead is the structural half. It catches a new
// raw os.Getenv anywhere in this package, not just at the one call site B35 was
// about. The only legitimate site is the accessor declaration itself.
func TestImage_TMUXHasExactlyOneRawEnvRead(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}

	fset := token.NewFileSet()
	var scanned int
	var sites []string

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

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "Getenv" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "os" {
				return true
			}
			sites = append(sites, name+":"+strconv.Itoa(fset.Position(call.Pos()).Line))
			return true
		})
	}

	// V2/V3: an enumeration that read nothing would make this vacuously green.
	// The package has multiple non-test files; zero means the walk is broken.
	if scanned == 0 {
		t.Fatal("scanned 0 non-test .go files in this package -- the walk is " +
			"broken, so a green result here would mean nothing")
	}

	// Exactly one raw read is correct: `var getenv = os.Getenv` in capability.go.
	// Note it is a function VALUE there, not a call, so it does not appear in
	// sites at all — which is why the expected count is zero, not one. If that
	// declaration is ever rewritten as a call, this test will say so.
	if len(sites) != 0 {
		t.Errorf("os.Getenv is CALLED at %d site(s) in package image: %s\n\n"+
			"This package reads the environment through `var getenv = os.Getenv`\n"+
			"(capability.go) so tests can inject values. A direct call bypasses\n"+
			"that: a test faking getenv sees its fake at one site and the real\n"+
			"environment at the other, and two capability decisions in one package\n"+
			"can disagree. This is gotcha B35, and AGENTS.md's rule for the class is\n"+
			"delete one, never alias -- so route the new site through getenv rather\n"+
			"than adding a second accessor.",
			len(sites), strings.Join(sites, ", "))
	}

	t.Logf("scanned %d non-test file(s); no direct os.Getenv calls", scanned)
}

// guard against the walk silently narrowing: capability.go must be among the
// files scanned above, since it is where the accessor lives.
func TestImage_StructuralWalkSeesCapabilityFile(t *testing.T) {
	if _, err := os.Stat(filepath.Join(".", "capability.go")); err != nil {
		t.Fatalf("capability.go is not where the structural guard expects it "+
			"(%v). If it moved, update TestImage_TMUXHasExactlyOneRawEnvRead's "+
			"reasoning about which file legitimately declares the accessor.", err)
	}
}
