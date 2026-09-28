package ui

import (
	"os"
	"testing"
)

// TestResolveEditor_UnsetDoesNotLeakPastItsSubtest guards against the leak
// fixed in editor_test.go's TestResolveEditor_FallsBackToConfigWhenNeitherEnvVarSet
// and TestResolveEditor_NoneConfiguredReturnsNotOk: a bare
// os.Unsetenv("VISUAL"/"EDITOR") with no restore left both variables
// permanently absent for every test that ran later in the same binary.
//
// It is self-contained rather than depending on the parent process's real
// environment: this test seeds PROBE_VAR itself with plain os.Setenv (so
// nothing auto-restores it) and cleans it up manually, then runs two
// sibling subtests in sequence. t.Setenv's restore-on-cleanup fires when
// the subtest that called it returns -- not when the parent test returns
// -- so a fixed pattern (t.Setenv arming a restore immediately before the
// bare Unsetenv) is already restored by the time the next sibling subtest
// runs, while the broken pattern (bare Unsetenv, no t.Setenv) is not.
//
// Verified directly: with the broken pattern substituted in the first
// subtest below, this test fails; with the fixed pattern (the one now
// used in editor_test.go), it passes.
func TestResolveEditor_UnsetDoesNotLeakPastItsSubtest(t *testing.T) {
	const key = "SLK_TEST_ENV_LEAK_PROBE"

	os.Setenv(key, "seed-value")
	t.Cleanup(func() { os.Unsetenv(key) })

	t.Run("unset_with_armed_restore", func(t *testing.T) {
		// This mirrors the fixed pattern in editor_test.go: t.Setenv
		// arms a restore-to-original cleanup, then the bare Unsetenv
		// (which the tests genuinely need, to get an UNSET var rather
		// than an empty one) runs on top of it.
		t.Setenv(key, "")
		os.Unsetenv(key)

		if v, ok := os.LookupEnv(key); ok {
			t.Fatalf("want %s unset inside this subtest, got %q", key, v)
		}
	})

	t.Run("probe_after", func(t *testing.T) {
		// If the previous subtest's Unsetenv leaked past its own scope
		// (i.e. armed no restore), the seed value is gone here too.
		v, ok := os.LookupEnv(key)
		if !ok || v != "seed-value" {
			t.Fatalf("%s leaked out of the prior subtest: want %q, got %q (present=%v)", key, "seed-value", v, ok)
		}
	})
}
