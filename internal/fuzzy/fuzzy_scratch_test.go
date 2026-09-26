package fuzzy

import (
	"testing"
)

func TestScratch(t *testing.T) {
	tier, score, ok := Match("male-doctor", "oc")
	t.Logf("male-doctor vs oc: tier=%v score=%v ok=%v\n", tier, score, ok)
	tier2, score2, ok2 := Match("popcorn", "oc")
	t.Logf("popcorn vs oc: tier=%v score=%v ok=%v\n", tier2, score2, ok2)
}
