package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gammons/slk/internal/cache"
	"github.com/gammons/slk/internal/config"
	slackclient "github.com/gammons/slk/internal/slack"
	"github.com/gammons/slk/internal/ui/sidebar"
	"github.com/slack-go/slack"
)

// fakeEnv adapts a map to the getenv closure parseLaunchArgs takes.
func fakeEnv(m map[string]string) func(string) string {
	return func(name string) string { return m[name] }
}

// runParse drives parseLaunchArgs with inert switch handlers and captures
// stdout/stderr, returning (action, exitCode, target).
func runParse(t *testing.T, argv []string, env map[string]string) (EarlyArgAction, int, *LaunchTarget, string, string) {
	t.Helper()
	var out, errB bytes.Buffer
	handlers := &switchHandlers{
		addWorkspace:    func() error { return nil },
		removeWorkspace: func() error { return nil },
		listWorkspaces:  func() error { return nil },
		dumpSections:    func() error { return nil },
		dumpPrefs:       func() error { return nil },
		hasWorkspaces:   func() bool { return true },
	}
	action, code, target, _ := parseLaunchArgs(argv, &out, &errB, fakeEnv(env), handlers)
	return action, code, target, out.String(), errB.String()
}

// --- cli-switch-compat: byte-identical early switches -----------------

func TestEarlySwitch_Compatibility(t *testing.T) {
	for _, argv := range [][]string{
		{"--version"}, {"-v"}, {"version"},
	} {
		action, code, _, out, _ := runParse(t, argv, nil)
		if action != EarlyArgHandled || code != 0 {
			t.Fatalf("%v: action=%v code=%d, want handled/0", argv, action, code)
		}
		want := "slk dev (commit none, built unknown)\n"
		if !strings.HasPrefix(out, want) {
			t.Errorf("%v: version output %q, want prefix %q", argv, out, want)
		}
	}
}

func TestEarlySwitch_HelpDoc(t *testing.T) {
	for _, argv := range [][]string{{"--help"}, {"-h"}, {"help"}} {
		action, code, _, out, _ := runParse(t, argv, nil)
		if action != EarlyArgHandled || code != 0 {
			t.Fatalf("%v: action=%v code=%d, want handled/0", argv, action, code)
		}
		if !strings.Contains(out, "--channel") || !strings.Contains(out, "SLK_FULLSCREEN") {
			t.Errorf("%v: help missing launch-pin docs:\n%s", argv, out)
		}
	}
}

func TestEarlySwitch_UnknownExits2WithoutTUI(t *testing.T) {
	action, code, _, _, errOut := runParse(t, []string{"--nonsense"}, nil)
	if action != EarlyArgHandled || code != 2 {
		t.Fatalf("action=%v code=%d, want handled/2", action, code)
	}
	if errOut == "" {
		t.Error("unknown flag produced no stderr")
	}
}

func TestEarlySwitch_BareLaunchNoArgs(t *testing.T) {
	action, code, target, _, _ := runParse(t, nil, nil)
	if action != EarlyArgLaunchTUI || code != 0 || target != nil {
		t.Fatalf("bare slk: action=%v code=%d target=%v, want TUI/0/nil", action, code, target)
	}
}

// --- cli-flags-parse ---------------------------------------------------

func TestParseLaunchArgs(t *testing.T) {
	cases := []struct {
		argv []string
		want func(t *testing.T, tgt *LaunchTarget)
	}{
		{[]string{"--channel", "C0123"}, func(t *testing.T, tgt *LaunchTarget) {
			if tgt.ChannelID != "C0123" || tgt.ChannelName != "" {
				t.Errorf("raw id: ChannelID=%q Name=%q", tgt.ChannelID, tgt.ChannelName)
			}
		}},
		{[]string{"--channel", "#general"}, func(t *testing.T, tgt *LaunchTarget) {
			if tgt.ChannelName != "general" {
				t.Errorf("name form: %q", tgt.ChannelName)
			}
		}},
		{[]string{"--channel", "https://acme.slack.com/archives/C0123/p1234567890123456"}, func(t *testing.T, tgt *LaunchTarget) {
			if tgt.ChannelID != "C0123" || tgt.WorkspaceHint != "acme" || tgt.SelectMessageTS == "" {
				t.Errorf("permalink: %+v", tgt)
			}
		}},
		{[]string{"--channel", "C0123", "--thread", "1234567890.123456"}, func(t *testing.T, tgt *LaunchTarget) {
			if tgt.ThreadTS != "1234567890.123456" {
				t.Errorf("thread pair: %q", tgt.ThreadTS)
			}
		}},
		{[]string{"--channel", "C0123", "--thread", "https://acme.slack.com/archives/C0123/p1234567890123456?thread_ts=1111111111.111111"}, func(t *testing.T, tgt *LaunchTarget) {
			if tgt.ThreadTS != "1111111111.111111" {
				t.Errorf("thread permalink: ThreadTS=%q", tgt.ThreadTS)
			}
		}},
		{[]string{"--fullscreen", "--channel", "C0123"}, func(t *testing.T, tgt *LaunchTarget) {
			if !tgt.Fullscreen {
				t.Error("fullscreen flag not set")
			}
		}},
	}
	for _, tc := range cases {
		action, code, tgt, _, _ := runParse(t, tc.argv, nil)
		if action != EarlyArgLaunchTUI || code != 0 || tgt == nil {
			t.Fatalf("%v: action=%v code=%d target=%v", tc.argv, action, code, tgt)
		}
		tc.want(t, tgt)
	}
}

func TestParseLaunch_MissingValueExits2(t *testing.T) {
	_, code, _, _, errOut := runParse(t, []string{"--channel"}, nil)
	if code != 2 || !strings.Contains(errOut, "flag needs an argument") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

func TestParseLaunchArgs_BareThreadWithoutChannel(t *testing.T) {
	_, code, _, _, errOut := runParse(t, []string{"--thread", "1234567890.123456"}, nil)
	if code != 2 || !strings.Contains(errOut, "--thread requires --channel") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

// lp-r10-env-merge: ENV thread without channel is a SOFT fallback.
func TestParseLaunchArgs_EnvFallback(t *testing.T) {
	action, code, tgt, _, _ := runParse(t, nil, map[string]string{"SLK_THREAD": "1234567890.123456"})
	if action != EarlyArgLaunchTUI || code != 0 {
		t.Fatalf("action=%v code=%d, want TUI/0", action, code)
	}
	if tgt == nil || !strings.Contains(tgt.Notice, "thread ignored") {
		t.Fatalf("target=%+v, want thread-ignored notice", tgt)
	}
}

// lp-r10-env-merge mixed case: SLK_CHANNEL=C1 + flag --thread resolves.
func TestParseLaunch_EnvChannelPlusFlagThread(t *testing.T) {
	action, code, tgt, _, _ := runParse(t, []string{"--thread", "1234567890.123456"}, map[string]string{"SLK_CHANNEL": "C1"})
	if action != EarlyArgLaunchTUI || code != 0 || tgt == nil {
		t.Fatalf("action=%v code=%d tgt=%v", action, code, tgt)
	}
	if tgt.ChannelID != "C1" || tgt.ThreadTS != "1234567890.123456" {
		t.Errorf("merged: %+v", tgt)
	}
}

// --- cli-env-equivalents: precedence flags > ENV -----------------------

func TestParseLaunch_Precedence(t *testing.T) {
	// flag wins over ENV
	_, _, tgt, _, _ := runParse(t, []string{"--channel", "CFLAG"}, map[string]string{"SLK_CHANNEL": "CENV"})
	if tgt.ChannelID != "CFLAG" {
		t.Errorf("flag+ENV: %q, want CFLAG", tgt.ChannelID)
	}
	// ENV only
	_, _, tgt, _, _ = runParse(t, nil, map[string]string{"SLK_CHANNEL": "CENV"})
	if tgt.ChannelID != "CENV" {
		t.Errorf("ENV only: %q, want CENV", tgt.ChannelID)
	}
	// empty/unset ENV indistinguishable from absent
	action, _, tgt, _, _ := runParse(t, nil, map[string]string{"SLK_CHANNEL": "   "})
	if action != EarlyArgLaunchTUI || tgt != nil {
		t.Errorf("whitespace ENV: action=%v tgt=%v, want TUI/nil", action, tgt)
	}
}

// lp-env-truthiness
func TestParseLaunch_FullscreenTruthiness(t *testing.T) {
	cases := []struct {
		val  string
		want bool
	}{
		{"1", true}, {"TRUE", true}, {"yes", true}, {"On", true},
		{"0", false}, {"false", false}, {"no", false},
		{"garbage", false}, {"   ", false},
	}
	for _, tc := range cases {
		_, _, tgt, _, _ := runParse(t, nil, map[string]string{"SLK_CHANNEL": "C1", "SLK_FULLSCREEN": tc.val})
		if tgt == nil || tgt.Fullscreen != tc.want {
			t.Errorf("SLK_FULLSCREEN=%q: Fullscreen=%v, want %v", tc.val, tgt, tc.want)
		}
	}
}

// lp-positional-reject
func TestParseLaunchArgs_PositionalArgsRejected(t *testing.T) {
	_, code, _, _, errOut := runParse(t, []string{"foo", "--channel", "X"}, nil)
	if code != 2 || !strings.Contains(errOut, "unexpected positional") {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

// lp-positional-reject, second half: a flag value starting with a dash
// (`--channel --fullscreen`) is a usage error, not a silently-swallowed pin.
func TestParseLaunchArgs_ValueStartingWithDash(t *testing.T) {
	_, code, _, _, errOut := runParse(t, []string{"--channel", "--fullscreen"}, nil)
	if code != 2 {
		t.Fatalf("code=%d err=%q", code, errOut)
	}
}

// lp-fsusage-wiring: --channel X --help exits 0 with the SAME help.
func TestParseLaunchArgs_HelpFlag(t *testing.T) {
	action, code, _, out, errOut := runParse(t, []string{"--channel", "C1", "--help"}, nil)
	if action != EarlyArgHandled || code != 0 {
		t.Fatalf("action=%v code=%d, want handled/0", action, code)
	}
	if errOut != "" {
		// lp-fsusage-wiring: the FlagSet's Usage callback must not print
		// (stdlib invokes it before returning ErrHelp); a second help
		// surface here doubles the text in the real binary.
		t.Errorf("stderr not empty on the help path: %q", errOut)
	}
	_, _, _, plain, _ := runParse(t, []string{"--help"}, nil)
	if out != plain {
		t.Error("--channel X --help output differs from --help output")
	}
}

// lp-cross-source-conflict: permalink channel wins + names both.
func TestParseLaunch_CrossSourceConflict(t *testing.T) {
	_, _, tgt, _, _ := runParse(t, []string{
		"--channel", "C9999",
		"--thread", "https://acme.slack.com/archives/C0123/p1234567890123456?thread_ts=1111111111.111111",
	}, nil)
	if tgt == nil || tgt.ChannelID != "C0123" {
		t.Fatalf("tgt=%+v, want permalink channel C0123 to win", tgt)
	}
	if tgt.ConflictNotice == "" || !strings.Contains(tgt.ConflictNotice, "C9999") {
		t.Errorf("conflict notice: %q", tgt.ConflictNotice)
	}
}

// lp-onboarding-drop: no workspaces configured → pin dropped silently.
func TestParseLaunch_OnboardingDrop(t *testing.T) {
	var out, errB bytes.Buffer
	handlers := &switchHandlers{
		hasWorkspaces: func() bool { return false },
	}
	action, code, tgt, _ := parseLaunchArgs([]string{"--channel", "C1"}, &out, &errB, nil, handlers)
	if action != EarlyArgLaunchTUI || code != 0 || tgt != nil {
		t.Errorf("pin not dropped: action=%v code=%d target=%+v", action, code, tgt)
	}
}

func TestParseLaunchArgs_NoConfiguredWorkspacesDropsPin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	var out, errB bytes.Buffer
	handlers := &switchHandlers{
		addWorkspace:    func() error { return nil },
		removeWorkspace: func() error { return nil },
		listWorkspaces:  func() error { return nil },
		dumpSections:    func() error { return nil },
		dumpPrefs:       func() error { return nil },
		// hasWorkspaces left nil so it uses hasConfiguredWorkspaces(xdgData())
	}
	action, code, tgt, _ := parseLaunchArgs([]string{"--channel", "C1"}, &out, &errB, nil, handlers)
	if action != EarlyArgLaunchTUI || code != 0 || tgt != nil {
		t.Errorf("pin not dropped with empty XDG_DATA_HOME: action=%v code=%d target=%+v", action, code, tgt)
	}
}

// R8: exactly one non-test read site for the SLK_* launch vars.
func TestLaunchEnvSingleReadSite(t *testing.T) {
	if env := os.Getenv("SLK_SINGLE_READ_SITE_SCAN"); env == "" {
		t.Skip("repo-level probe; see TestLaunchEnvReadForm (form probe below)")
	}
}

// Appendix (cli-env-equivalents): probe the read FORM — files containing
// an os.Getenv("SLK_...") call, excluding tests — exactly one.
func TestLaunchEnvReadForm(t *testing.T) {
	if testing.Short() {
		t.Skip("source scan")
	}
	root := "."
	var hits []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() && info.Name() == ".git" {
				return filepath.SkipDir
			}
			return err
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), `os.Getenv("SLK_CHANNEL")`) ||
			strings.Contains(string(data), `os.Getenv("SLK_THREAD")`) ||
			strings.Contains(string(data), `os.Getenv("SLK_FULLSCREEN")`) {
			hits = append(hits, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(hits) != 1 || hits[0] != "launch_pin.go" {
		t.Errorf("os.Getenv(SLK_*) read sites = %v, want exactly [launch_pin.go]", hits)
	}
}

// --- target-workspace-route + domain source ---------------------------

func TestMatchWorkspaceBySubdomain(t *testing.T) {
	tokens := []slackclient.Token{
		{TeamID: "T1", Domain: "acme-corp"},
		{TeamID: "T2", Domain: "OTHER"},
	}
	// Mixed-case URL host side must fold.
	if id, ok := matchWorkspaceBySubdomain("ACME-CORP", tokens, nil); !ok || id != "T1" {
		t.Errorf("token-domain match: id=%q ok=%v, want T1", id, ok)
	}
	// Cache fallback when token Domain is empty.
	cacheWs := []cache.Workspace{{ID: "T3", Domain: "cached"}}
	if id, ok := matchWorkspaceBySubdomain("cached", []slackclient.Token{{TeamID: "T4"}}, cacheWs); !ok || id != "T3" {
		t.Errorf("cache fallback: id=%q ok=%v, want T3", id, ok)
	}
	// Unmatched -> false.
	if _, ok := matchWorkspaceBySubdomain("nope", tokens, cacheWs); ok {
		t.Error("unmatched subdomain matched")
	}
	// Empty-Domain token never matches (lp-workspace-domain-source probe).
	if _, ok := matchWorkspaceBySubdomain("", []slackclient.Token{{TeamID: "T5", Domain: ""}}, nil); ok {
		t.Error("empty subdomain matched an empty-domain token")
	}
}

// --- target-channel-resolve (R11) --------------------------------------

func TestResolveChannelTarget(t *testing.T) {
	channels := []sidebar.ChannelItem{
		{ID: "C1", Name: "general", Type: "channel"},
		{ID: "C2", Name: "random", Type: "channel"},
		{ID: "D1", Name: "alice", Type: "dm"},
	}
	// ID hit
	tgt := &LaunchTarget{ChannelID: "C1"}
	if id, _, notice := resolveChannelTarget(context.Background(), tgt, channels, nil); id != "C1" || notice != "" {
		t.Errorf("id hit: id=%q notice=%q", id, notice)
	}
	// ID miss + no client -> notice, no exit
	tgt = &LaunchTarget{ChannelID: "CGONE"}
	if id, _, notice := resolveChannelTarget(context.Background(), tgt, channels, nil); id != "" || !strings.Contains(notice, "not found") {
		t.Errorf("id miss: id=%q notice=%q", id, notice)
	}
	// ID miss + erroring client -> notice, fallback
	tgt = &LaunchTarget{ChannelID: "CGONE"}
	errFn := func(ctx context.Context, id string) (*slack.Channel, error) {
		return nil, errors.New("boom")
	}
	if id, _, notice := resolveChannelTarget(context.Background(), tgt, channels, errFn); id != "" || !strings.Contains(notice, "not found") {
		t.Errorf("id miss+err: id=%q notice=%q", id, notice)
	}
	// ID miss + hit client -> resolved + unlisted conv (hydrate path)
	tgt = &LaunchTarget{ChannelID: "CARCH"}
	hitFn := func(ctx context.Context, id string) (*slack.Channel, error) {
		return &slack.Channel{GroupConversation: slack.GroupConversation{Conversation: slack.Conversation{ID: "CARCH"}, Name: "archived-chan"}}, nil
	}
	id, unlisted, notice := resolveChannelTarget(context.Background(), tgt, channels, hitFn)
	if id != "CARCH" || unlisted == nil || unlisted.ID != "CARCH" || notice != "" {
		t.Errorf("unlisted: id=%q unlisted=%v notice=%q", id, unlisted, notice)
	}
	// Name hit, case/accent-folded
	tgt = &LaunchTarget{ChannelName: "GENERAL"}
	if id, _, notice := resolveChannelTarget(context.Background(), tgt, channels, nil); id != "C1" || notice != "" {
		t.Errorf("fold name: id=%q notice=%q", id, notice)
	}
	tgt = &LaunchTarget{ChannelName: "general"}
	if id, _, _ := resolveChannelTarget(context.Background(), tgt, channels, nil); id != "C1" {
		t.Errorf("name: id=%q", id)
	}
	// DM display names must NOT match (name forms resolve channels only)
	tgt = &LaunchTarget{ChannelName: "alice"}
	if id, _, notice := resolveChannelTarget(context.Background(), tgt, channels, nil); id != "" || !strings.Contains(notice, "not found") {
		t.Errorf("dm name: id=%q notice=%q", id, notice)
	}
	// Ambiguity -> notice, not first-match
	channels2 := []sidebar.ChannelItem{
		{ID: "C1", Name: "dup", Type: "channel"},
		{ID: "C2", Name: "DUP", Type: "channel"},
	}
	tgt = &LaunchTarget{ChannelName: "dup"}
	if id, _, notice := resolveChannelTarget(context.Background(), tgt, channels2, nil); id != "" || !strings.Contains(notice, "ambiguous") {
		t.Errorf("ambiguous: id=%q notice=%q", id, notice)
	}
}

// lp-resolve-timeout: a client that sleeps past the deadline falls back.
func TestResolveChannelTarget_SlowClientTimesOut(t *testing.T) {
	if testing.Short() {
		t.Skip("3s timeout path")
	}
	channels := []sidebar.ChannelItem{{ID: "C1", Name: "general", Type: "channel"}}
	tgt := &LaunchTarget{ChannelID: "CWEDGE"}
	slow := func(ctx context.Context, id string) (*slack.Channel, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return &slack.Channel{}, nil
		}
	}
	start := time.Now()
	id, _, notice := resolveChannelTarget(context.Background(), tgt, channels, slow)
	if id != "" || !strings.Contains(notice, "not found") {
		t.Errorf("slow: id=%q notice=%q", id, notice)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("slow client not bounded: %v", elapsed)
	}
}

// target-pure-parse, target-thread-forms, lp-cross-source-conflict, lp-pin-provenance
func TestResolveLaunchTarget_Pure(t *testing.T) {
	// Permalink with channel and thread_ts
	plThread := "https://myworkspace.slack.com/archives/C0123ABCD/p1700000000000001?thread_ts=1700000000.000099"
	t1, err := resolveLaunchTarget(plThread, "", false, "flag", "", "")
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if t1.WorkspaceHint != "myworkspace" {
		t.Errorf("WorkspaceHint = %q, want myworkspace", t1.WorkspaceHint)
	}
	if t1.ChannelID != "C0123ABCD" {
		t.Errorf("ChannelID = %q, want C0123ABCD", t1.ChannelID)
	}
	if t1.ThreadTS != "1700000000.000099" {
		t.Errorf("ThreadTS = %q, want 1700000000.000099", t1.ThreadTS)
	}

	// Permalink without thread_ts (select-message form)
	plMsg := "https://myworkspace.slack.com/archives/C0123ABCD/p1700000000000001"
	t2, err := resolveLaunchTarget(plMsg, "", false, "flag", "", "")
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if t2.SelectMessageTS != "1700000000.000001" {
		t.Errorf("SelectMessageTS = %q, want 1700000000.000001", t2.SelectMessageTS)
	}
	if t2.ThreadTS != "" {
		t.Errorf("ThreadTS should be empty for select-message permalink, got %q", t2.ThreadTS)
	}

	// Conflict: thread permalink carries C1, while explicit channel names C2
	plConflict := "https://myworkspace.slack.com/archives/C11111111/p1700000000000001?thread_ts=1700000000.000099"
	t3, err := resolveLaunchTarget("C22222222", plConflict, false, "flag", "flag", "")
	if err != nil {
		t.Fatalf("resolve conflict error: %v", err)
	}
	if t3.ChannelID != "C11111111" {
		t.Errorf("Permalink channel must win on conflict, got %q", t3.ChannelID)
	}
	if !strings.Contains(t3.ConflictNotice, "overrides") {
		t.Errorf("Conflict notice missing or incomplete: %q", t3.ConflictNotice)
	}

	// Provenance notice for ENV pin
	tEnv, err := resolveLaunchTarget("general", "", false, "env", "", "")
	if err != nil || !strings.Contains(tEnv.Notice, "pinned to #general via SLK_CHANNEL") {
		t.Errorf("ENV provenance notice missing: %+v", tEnv)
	}
}

// lp-zoom-decision-two-layer CLI layer probe (R3)
func TestZoomDecisionTwoLayer_CLI(t *testing.T) {
	ctx := context.Background()
	channels := []sidebar.ChannelItem{
		{ID: "C_OK", Name: "general", Type: "channel"},
	}

	// Case A: channel resolved, thread missing, fullscreen requested ->
	// target channel is resolved, thread is empty, fullscreen is preserved (promotes channel)
	targetA := &LaunchTarget{ChannelName: "general", ThreadTS: "", Fullscreen: true}
	idA, _, noticeA := resolveChannelTarget(ctx, targetA, channels, nil)
	if idA != "C_OK" || noticeA != "" {
		t.Errorf("Case A resolve: id=%q notice=%q", idA, noticeA)
	}

	// Case B: neither resolved (channel unresolvable), fullscreen requested ->
	// channel is empty, fullscreen must be dropped per R3
	targetB := &LaunchTarget{ChannelName: "nonexistent", Fullscreen: true}
	idB, _, noticeB := resolveChannelTarget(ctx, targetB, channels, nil)
	if idB != "" || !strings.Contains(noticeB, "not found") {
		t.Errorf("Case B resolve: id=%q notice=%q", idB, noticeB)
	}
	// Verify resolveAndHydrateLaunchTarget drops fullscreen when channel fails to resolve
	wctx := &WorkspaceContext{
		Channels: channels,
	}
	cfg := config.Config{}
	pinChID, _, _, pinFullscreen, _ := resolveAndHydrateLaunchTarget(ctx, targetB, wctx, cfg)
	if pinChID != "" || pinFullscreen {
		t.Errorf("Case B resolveAndHydrate: pinChID=%q pinFullscreen=%v, want empty and false", pinChID, pinFullscreen)
	}
}
