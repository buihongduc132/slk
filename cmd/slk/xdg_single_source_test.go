package main

// B41 oracle: XDG_DATA_HOME must have exactly ONE resolution rule.
//
// This file is the gate oracle for lane-xdg and is HASH-PINNED. Do not edit it
// to make it pass -- the whole point of a pinned oracle is that the code moves
// and the question does not. If you believe the question is wrong, say so in a
// report and leave the gate red; that is a successful outcome for this lane.
//
// The defect, verified before this file was written:
//
//	cmd/slk/paths.go:17        if dir := os.Getenv("XDG_DATA_HOME"); dir != "" { ... }
//	internal/export/markdown.go:71   if dir := os.Getenv("XDG_DATA_HOME"); dir != "" { ... }
//
// Two independent reads of one variable, each with its own unset-case fallback.
// This is the mandated gotcha class ("one value read in more than one place"),
// whose resolution is DELETE ONE, NEVER ALIAS: a second site that merely
// delegates to the first still reads the environment twice and still drifts.
//
// The divergence is not hypothetical. It is already observable on the
// home-directory error path, which is what TestXDG_DataAndExportAgreeWhenHomeIsUnset
// pins:
//
//	xdgData()    does `home, _ := os.UserHomeDir()` -- SWALLOWS the error, so it
//	             returns filepath.Join("", ".local", "share", "slk"), i.e. the
//	             RELATIVE path ".local/share/slk". Writes land in the process's
//	             current directory, wherever that happens to be.
//	ExportDir()  propagates the error and refuses.
//
// So with no HOME and no XDG_DATA_HOME, slk's data root and its export root do
// not merely differ, they disagree about whether the operation is possible.
//
// Note for whoever fixes this: internal/export cannot import cmd/slk, which is
// why the rule was re-derived in the first place. The shared resolver therefore
// has to live in a third, lower-level package that both can import.
//
// The error-handling contract is a REAL DECISION and not one this oracle makes
// for you. xdgData() returns a bare string (11 call sites rely on that);
// ExportDir() returns (string, error) (1 call site relies on that). Unifying the
// rule does not require unifying the signatures -- one resolver plus an explicit
// swallow at the cmd/slk boundary keeps both contracts while leaving exactly one
// rule. What this oracle forbids is two rules. What it requires on the error
// path is that the two answers agree.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gammons/slk/internal/export"
)

// TestXDG_DataHomeHasExactlyOneResolutionSite is structural, deliberately.
//
// No behavioural test can see the difference between "one rule" and "two rules
// that currently agree" -- that is precisely why this defect survived: both
// sites produce the same string on the happy path, so every passing test stayed
// passing. The thing that is wrong here is the SHAPE of the code, so the shape
// is what gets asserted. (Compare internal/ui/zoom_lastrow_hittest_test.go's
// TestZoom_MouseClickHasNoOwnStatusRowLiteral, which exists for the same
// reason.)
//
// It walks the repo rather than naming the two known files on purpose: a third
// site added later must fail this too.
func TestXDG_DataHomeHasExactlyOneResolutionSite(t *testing.T) {
	root := repoRootForXDGTest(t)

	var sites []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata", "wiki", "docs", "flow":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		// The env var name as it appears in a Getenv call. Comments mentioning
		// it are fine -- only a read counts, so match on the quoted literal
		// next to Getenv rather than on the bare name.
		for _, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, `Getenv("XDG_DATA_HOME")`) {
				rel, _ := filepath.Rel(root, path)
				sites = append(sites, rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	if len(sites) != 1 {
		t.Errorf("XDG_DATA_HOME is read in %d non-test site(s), want exactly 1: %v\n"+
			"Resolution for this gotcha class is DELETE ONE, NEVER ALIAS. A second\n"+
			"site that delegates to the first still reads the environment twice.",
			len(sites), sites)
	}
}

// TestXDG_DataAndExportAgreeWhenHomeIsUnset is the behavioural half, and it is
// the one that proves the defect is user-visible rather than merely untidy.
//
// With neither XDG_DATA_HOME nor HOME set, the two rules disagree about whether
// a data directory can be determined at all. Whatever the fix decides, the two
// answers have to agree afterwards: either both refuse, or both produce the same
// root. This test does not dictate which.
// A NOTE ON THE PREDICATE, because the first version of this test PASSED and
// was worthless:
//
// It defined "usable" as `path != "" && filepath.IsAbs(path)` for BOTH sides.
// That made both sides unusable -- xdgData()'s relative ".local/share/slk" fails
// IsAbs, and ExportDir() errors -- so the two "agreed" and the test went green
// over the top of the exact divergence it was written to catch. Abstracting both
// answers through one predicate erased the difference between them.
//
// The asymmetry is the whole point and it has to stay visible: xdgData() has NO
// ERROR CHANNEL, so any non-empty string it returns is consumed by all 11 of its
// call sites without further checking (`filepath.Join(xdgData(), "tokens")` and
// friends). ExportDir() has one and uses it. So "did resolution succeed" means
// something different on each side, and that is what gets compared.
func TestXDG_DataAndExportAgreeWhenHomeIsUnset(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")

	data := xdgData()
	exp, expErr := export.ExportDir()

	// Success, per each API's own contract.
	dataResolved := data != ""   // no error channel: non-empty means "callers will use this"
	expResolved := expErr == nil // has an error channel and uses it

	if dataResolved != expResolved {
		t.Errorf("the two XDG_DATA_HOME rules disagree about whether resolution succeeded, HOME unset:\n"+
			"  xdgData()   = %q  -> resolved=%v (no error channel; 11 call sites join onto this and write)\n"+
			"  ExportDir() = %q, err=%v -> resolved=%v\n"+
			"After the fix the two must agree: both refuse, or both resolve.",
			data, dataResolved, exp, expErr, expResolved)
	}

	// Independently of agreement: a data root that callers will join onto and
	// write into MUST be absolute. A relative one silently puts slk's tokens
	// and cache in whatever directory the process happens to be started from.
	if data != "" && !filepath.IsAbs(data) {
		t.Errorf("xdgData() = %q -- RELATIVE, with no error returned.\n"+
			"Callers do filepath.Join(xdgData(), \"tokens\") and write there, so this\n"+
			"puts credentials under the process's current directory. Either return an\n"+
			"absolute path or give the function a way to say it cannot.", data)
	}

	// If both resolve, the export root must sit under the data root -- the
	// relationship the happy path already has, which must not be lost.
	if dataResolved && expResolved {
		if want := filepath.Join(data, "exports"); exp != want {
			t.Errorf("ExportDir() = %q, want %q (under the data root)", exp, want)
		}
	}
}

// TestXDG_ExportDirIsUnderDataDir pins the happy-path relationship that already
// holds, so that a fix which unifies the error path cannot quietly relocate
// exports while doing it. This is a regression guard, GREEN before and after.
func TestXDG_ExportDirIsUnderDataDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)

	data := xdgData()
	exp, err := export.ExportDir()
	if err != nil {
		t.Fatalf("ExportDir() with XDG_DATA_HOME=%q: %v", tmp, err)
	}
	if want := filepath.Join(data, "exports"); exp != want {
		t.Errorf("ExportDir() = %q, want %q\n"+
			"(xdgData() = %q; exports must live under the data root, and this is\n"+
			"the invariant the two independent rules happened to satisfy)",
			exp, want, data)
	}
	if !strings.HasPrefix(exp, tmp) {
		t.Errorf("ExportDir() = %q, want it under XDG_DATA_HOME=%q", exp, tmp)
	}
}

// repoRootForXDGTest finds the module root by walking up for go.mod. Tests run
// with the package directory as cwd, so this is cmd/slk -> repo root.
func repoRootForXDGTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod found walking up from the test's working directory")
		}
		dir = parent
	}
}
