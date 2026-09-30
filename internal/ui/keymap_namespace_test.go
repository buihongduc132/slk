package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
)

// Keymap namespace guard (herdr coexistence contract).
//
// slk runs inside herdr panes. herdr claims a GLOBAL, direct (no-prefix)
// slice of the bare-ctrl space for itself — ctrl+h/j/k/l (pane focus),
// ctrl+s / ctrl+d (splits), ctrl+z (zoom), ctrl+x (copy mode), ctrl+p /
// ctrl+n (prev/next workspace), ctrl+1..9 (tab switch) — plus its prefix
// ctrl+u and alt+u / alt+i. Because herdr sees every key before the pane
// does, ANY bare-ctrl binding slk declares is dead on arrival under
// herdr: herdr eats it and slk never fires.
//
// The resolution (user requirement, 2026-09-26): slk's entire
// ctrl-modified surface lives in the ctrl+alt+<key> namespace, which
// herdr, wezTerm and the dy-plane stack leave untouched.
//
// This test is the codified rule. Adding a bare-ctrl binding anywhere in
// DefaultKeyMap fails here at CI time — the sibling of the bug this
// family of migrations killed.

// herdrDirectKeys is the set of key strings herdr claims globally, taken
// from the dy-plane herdr config ([keys] table). Keep in sync with
// prompts/components/dy-plane/ops/main-controller/herdr/config.toml.
var herdrDirectKeys = map[string]bool{
	// pane focus (direct bindings)
	"ctrl+h": true, "ctrl+j": true, "ctrl+k": true, "ctrl+l": true,
	// splits / zoom / copy mode
	"ctrl+s": true, "ctrl+d": true, "ctrl+z": true, "ctrl+x": true,
	// workspace prev/next
	"ctrl+p": true, "ctrl+n": true,
	// herdr prefix
	"ctrl+u": true,
	// tab switching without prefix
	"ctrl+1": true, "ctrl+2": true, "ctrl+3": true, "ctrl+4": true,
	"ctrl+5": true, "ctrl+6": true, "ctrl+7": true, "ctrl+8": true,
	"ctrl+9": true, "ctrl+0": true,
	// direct alt bindings (tmux M- heritage)
	"alt+u": true, "alt+i": true,
}

// isBareCtrl reports whether a key string is ctrl-modified WITHOUT alt:
// "ctrl+x" and "ctrl+shift+y" yes; "ctrl+alt+z" no; "alt+x" no.
func isBareCtrl(k string) bool {
	return strings.HasPrefix(k, "ctrl+") && !strings.HasPrefix(k, "ctrl+alt+")
}

func TestKeyMap_NoBareCtrlBindings(t *testing.T) {
	km := DefaultKeyMap()
	for name, binding := range keyMapBindings(km) {
		for _, k := range binding.Keys() {
			if isBareCtrl(k) {
				t.Errorf("%s: key %q is a bare-ctrl binding — herdr owns the bare-ctrl space; use ctrl+alt+<key>", name, k)
			}
		}
	}
}

func TestKeyMap_HerdrDisjoint(t *testing.T) {
	km := DefaultKeyMap()
	for name, binding := range keyMapBindings(km) {
		for _, k := range binding.Keys() {
			if herdrDirectKeys[k] {
				t.Errorf("%s: key %q is claimed globally by herdr — it can never fire inside a herdr pane", name, k)
			}
		}
	}
}

func TestKeyMap_CtrlAltKeysUnique(t *testing.T) {
	km := DefaultKeyMap()
	seen := map[string]string{}
	for name, binding := range keyMapBindings(km) {
		for _, k := range binding.Keys() {
			if !strings.HasPrefix(k, "ctrl+alt+") {
				continue
			}
			if prev, dup := seen[k]; dup {
				t.Errorf("ctrl+alt key %q bound twice: %s and %s", k, prev, name)
			}
			seen[k] = name
		}
	}
}

// TestKeyMap_HelpMatchesKeys pins that the help text of every migrated
// binding names the same key the binding actually matches — a stale
// "ctrl+f" help string on a ctrl+alt+f binding is a documentation bug
// this family of migrations produced once already.
func TestKeyMap_HelpMatchesKeys(t *testing.T) {
	km := DefaultKeyMap()
	for name, binding := range keyMapBindings(km) {
		ks := binding.Keys()
		if len(ks) == 0 {
			continue // keyless help-only entries (WorkspaceFinder etc.)
		}
		help := binding.Help().Key
		for _, part := range strings.Split(help, "/") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			// Help strings may name fallbacks that are not bindings
			// (":ws", "alt+enter", "gg"); only pin the ctrl-modified
			// ones, which are the migrated surface.
			if !strings.HasPrefix(part, "ctrl+") {
				continue
			}
			if !isBareCtrl(part) {
				continue // already ctrl+alt+… — fine
			}
			// A bare-ctrl help fragment must correspond to a real
			// legacy alias still present in Keys(); otherwise the help
			// advertises a dead binding.
			found := false
			for _, k := range ks {
				if k == part {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: help names %q but no such key remains bound (stale help from the pre-migration keymap)", name, part)
			}
		}
	}
}

// keyMapBindings reflects over the KeyMap struct, returning
// field name → binding. REAL reflection (reflect.Value.FieldByName over
// the struct type): a NEW KeyMap field is covered the moment it is
// added — this guard cannot go stale by omission. A non-key.Binding
// field would fail the type assertion and break the test, which is the
// correct failure (KeyMap is a flat binding struct by design).
func keyMapBindings(km KeyMap) map[string]key.Binding {
	out := map[string]key.Binding{}
	v := reflect.ValueOf(km)
	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		b, ok := v.Field(i).Interface().(key.Binding)
		if !ok {
			panic(fmt.Sprintf("KeyMap field %s is not a key.Binding — KeyMap must stay a flat binding struct", f.Name))
		}
		out[f.Name] = b
	}
	return out
}

// TestKeyMap_GuardCoversEveryField pins that the reflection-based
// guard enumerates EVERY field of KeyMap — the mechanism the earlier
// hand-maintained add() list faked. If someone renames the reflect
// walk to a subset (or reintroduces a hand list), the count diverges
// and this fails before any binding slips through uncovered.
func TestKeyMap_GuardCoversEveryField(t *testing.T) {
	km := DefaultKeyMap()
	got := keyMapBindings(km)
	typ := reflect.TypeOf(km)
	if len(got) != typ.NumField() {
		t.Fatalf("guard covers %d bindings, KeyMap has %d fields — coverage hole", len(got), typ.NumField())
	}
}
