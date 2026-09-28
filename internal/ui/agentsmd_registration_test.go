package ui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestAGENTSMD_EveryTableRowNamesSomethingThatExists turns AGENTS.md's tables
// from prose into a checked claim.
//
// AGENTS.md opens with "the single most important rule: search before you
// write", and its tables are the index that rule depends on. An index is only
// useful while it is true: a row naming a helper that has since been renamed or
// deleted sends the next author looking for something that is not there, and
// they write a fresh copy — which is the exact defect the file exists to
// prevent. Nothing enforced that the rows still resolved, so this does.
//
// WHAT IT ASSERTS: every backticked, identifier-shaped token in every markdown
// table row of AGENTS.md resolves to a name that exists in this module — a
// declaration (func, method, type, const, var, struct field), a package
// directory, or a string literal in the source.
//
// WHY THE THREE SOURCES: the tables legitimately name more than test helpers.
// Rows name production fields the helpers set (threadVisible, yOffset,
// forceSixelRepaint), packages rather than symbols (threadsview,
// newmessagepicker), and one external binary (pbcopy, in the clipboard row) that
// exists only as an argument to exec.Command. Measured over the current file:
// 121 tokens are checked, exactly one of which — pbcopy — resolves through the
// string-literal source alone. The literal set is broad (22k values), so it is
// the weakest of the three; that is a deliberate trade. For a documentation
// guard a false RED is costly and a false GREEN is merely a missed catch, and
// the failure this is aimed at — a name that has left the module entirely — is
// caught by all three sources going quiet at once.
//
// WHAT IT DOES NOT ASSERT, and cannot:
//
//   - That a listed helper still does what its row says. The row's prose is not
//     checkable this way; only the name is.
//   - That every reusable helper IS listed. That is the other half of the
//     Conventions rule ("Adding a reusable helper? Add it to the tables above in
//     the same commit") and is the direction that actually caused duplication
//     here — eight helpers existed unlisted, three of them written twice. But
//     "reusable" has no mechanical definition: asserting it would need a curated
//     list of which of the module's 4,095 test-declared names deserve a row,
//     which is the same hand-maintained artefact this file is trying to retire.
//     Left to review.
//   - Name collisions. A helper deleted today still resolves if any struct field
//     anywhere shares its identifier. Accepted, for the same reason as above.
//
// If this test fails, the fix is normally to edit AGENTS.md, not the code: the
// row is describing something that no longer exists. Per the Conventions
// section — "when this file and the code disagree, the code is right and this
// file is a bug".
func TestAGENTSMD_EveryTableRowNamesSomethingThatExists(t *testing.T) {
	root := agentsMDModuleRoot(t)
	universe := newModuleUniverse(t, root)
	tokens := agentsMDTableTokens(t, root)

	// Vacuity floor. Every count below is one a broken walk or a broken parse
	// would drive to zero while the assertions below stayed silent, which is the
	// shape of failure this repo has hit most often.
	if universe.files == 0 {
		t.Fatal("parsed 0 .go files: the module walk is broken, any verdict here would be vacuous")
	}
	if len(universe.names) == 0 {
		t.Fatal("collected 0 declared names: the AST walk is broken")
	}
	if len(tokens) == 0 {
		t.Fatal("extracted 0 tokens from AGENTS.md tables: the markdown parse is broken")
	}

	var checked, unresolved []string
	for _, tok := range tokens {
		bare, skip := universe.resolvable(tok, tokens)
		if skip {
			continue
		}
		checked = append(checked, tok)
		if !universe.has(bare) {
			unresolved = append(unresolved, tok)
		}
	}

	// A floor, not a target: the measured count when this was written was 121.
	// Half of that is low enough never to fire on ordinary table edits and high
	// enough to catch an extraction that has silently stopped finding rows.
	const minChecked = 60
	if len(checked) < minChecked {
		t.Fatalf("only %d tokens were checked (floor %d): the extraction or the skip rules "+
			"have stopped seeing the tables, so a PASS would mean nothing", len(checked), minChecked)
	}

	for _, tok := range unresolved {
		t.Errorf("AGENTS.md names %q, which no longer exists anywhere in this module "+
			"(no declaration, no package directory, no string literal). The row is stale: "+
			"fix AGENTS.md, or restore the name.", tok)
	}
	t.Logf("checked %d tokens from AGENTS.md tables against %d declared names, %d package dirs, %d literals (%d files)",
		len(checked), len(universe.names), len(universe.dirs), len(universe.literals), universe.files)
}

// moduleUniverse is every name this module mentions, in the three forms an
// AGENTS.md row may legitimately be naming.
type moduleUniverse struct {
	names    map[string]bool // funcs, methods, types, consts, vars, struct fields
	dirs     map[string]bool // base names of directories holding .go files
	literals map[string]bool // string-literal values (external commands, keys)
	imports  map[string]bool // usable package qualifiers: last path element or alias
	files    int
}

func (u *moduleUniverse) has(name string) bool {
	return u.names[name] || u.dirs[name] || u.literals[name]
}

// agentsMDPredeclared are the predeclared identifiers a row may name as a type
// or value. They are not declared in this module, so they must be skipped rather
// than resolved.
var agentsMDPredeclared = map[string]bool{
	"uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"int8": true, "int16": true, "int32": true, "int64": true,
	"int": true, "uint": true, "uintptr": true, "byte": true, "rune": true,
	"string": true, "bool": true, "error": true, "any": true,
	"float32": true, "float64": true, "complex64": true, "complex128": true,
	"true": true, "false": true, "nil": true, "iota": true,
}

// resolvable decides what, if anything, a table token promises about this
// module, returning the bare name to look up and whether to skip it entirely.
//
// The skip rules are derived from the code and from the file itself, never from a
// hand-maintained exception list:
//
//   - a predeclared identifier, or a *.go filename, promises nothing;
//   - `tea.Batch`, `ansi.Strip`, `testing.TB`: the qualifier is an import name
//     somewhere in this module, so the symbol belongs to a foreign package;
//   - `WindowSizeMsg`: a bare token that ALSO appears package-qualified in these
//     same tables (`tea.WindowSizeMsg`, line 140) is that foreign type mentioned
//     without its qualifier. The table supplies the evidence, so no exception is
//     needed;
//   - `a.setChannelFetcherForTest`: the qualifier `a` is NOT an import name — it
//     is the App receiver — so this is one of the module's own methods and IS
//     checked, by its bare name.
func (u *moduleUniverse) resolvable(tok string, all []string) (bare string, skip bool) {
	if agentsMDPredeclared[tok] || strings.HasSuffix(tok, ".go") {
		return "", true
	}
	if i := strings.LastIndex(tok, "."); i > 0 {
		if u.imports[tok[:i]] {
			return "", true
		}
		return tok[i+1:], false
	}
	for _, other := range all {
		if i := strings.LastIndex(other, "."); i > 0 && other[i+1:] == tok && u.imports[other[:i]] {
			return "", true
		}
	}
	return tok, false
}

// newModuleUniverse parses every .go file in the module — test and non-test, one
// pass — collecting names, imports and string literals.
//
// Nested modules are pruned. This repo keeps agent worktrees under
// .claude/worktrees/, each a full copy of the tree carrying its own go.mod; the
// copies would not change any verdict here (they hold the same names) but they
// multiply the walk by the number of live worktrees. Same remedy as the clock
// guard and the xdg oracle, for the same reason.
func newModuleUniverse(t *testing.T, root string) *moduleUniverse {
	t.Helper()

	u := &moduleUniverse{
		names:    map[string]bool{},
		dirs:     map[string]bool{},
		literals: map[string]bool{},
		imports:  map[string]bool{},
	}
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			if d.Name() == ".git" || d.Name() == "testdata" {
				return fs.SkipDir
			}
			if _, statErr := os.Stat(filepath.Join(path, "go.mod")); statErr == nil {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		u.files++
		u.dirs[filepath.Base(filepath.Dir(path))] = true

		for _, imp := range f.Imports {
			if imp.Name != nil {
				u.imports[imp.Name.Name] = true
				continue
			}
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				continue
			}
			u.imports[p[strings.LastIndex(p, "/")+1:]] = true
		}

		ast.Inspect(f, func(nd ast.Node) bool {
			switch v := nd.(type) {
			case *ast.FuncDecl:
				u.names[v.Name.Name] = true
			case *ast.TypeSpec:
				u.names[v.Name.Name] = true
			case *ast.ValueSpec:
				for _, id := range v.Names {
					u.names[id.Name] = true
				}
			case *ast.Field:
				for _, id := range v.Names {
					u.names[id.Name] = true
				}
			case *ast.BasicLit:
				if v.Kind == token.STRING {
					if s, uerr := strconv.Unquote(v.Value); uerr == nil {
						u.literals[s] = true
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking the module: %v", err)
	}
	return u
}

var (
	agentsMDTicked = regexp.MustCompile("`([^`]+)`")
	agentsMDIdent  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)
)

// agentsMDTableTokens extracts every backticked, identifier-shaped token from
// every markdown table row in AGENTS.md.
//
// Table rows only: the prose around them names types, packages and concepts
// loosely, and holding prose to this standard would produce false REDs on
// sentences that are perfectly accurate. A row, by contrast, exists to tell the
// reader "call this".
//
// A trailing call or argument list is trimmed (`WordWrap(s, limit)` → WordWrap),
// so a row may keep its illustrative signature. Anything still holding a space,
// a slash or a path separator after that is prose, not an identifier, and is
// dropped by agentsMDIdent.
func agentsMDTableTokens(t *testing.T, root string) []string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading AGENTS.md: %v", err)
	}

	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "| ") ||
			strings.HasPrefix(line, "| Need") ||
			strings.HasPrefix(line, "|---") {
			continue
		}
		for _, m := range agentsMDTicked.FindAllStringSubmatch(line, -1) {
			tok := strings.TrimSpace(m[1])
			if i := strings.Index(tok, "("); i > 0 {
				tok = tok[:i]
			}
			if !agentsMDIdent.MatchString(tok) || seen[tok] {
				continue
			}
			seen[tok] = true
			out = append(out, tok)
		}
	}
	sort.Strings(out)
	return out
}

// agentsMDModuleRoot walks up from the test's working directory to the directory
// holding go.mod. internal/ui/messages has its own copy (moduleRoot, in
// clock_parallel_guard_test.go); unexported helpers are not reachable across
// packages, so this is a second declaration rather than a duplicate to
// consolidate. Registered in AGENTS.md alongside it.
func agentsMDModuleRoot(t *testing.T) string {
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
			t.Fatalf("no go.mod found walking up from %s", dir)
		}
		dir = parent
	}
}
