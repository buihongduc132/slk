package emojipicker

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/emoji"
	"github.com/gammons/slk/internal/fuzzy"
	imgpkg "github.com/gammons/slk/internal/image"
	"github.com/gammons/slk/internal/text"
	"github.com/gammons/slk/internal/ui/styles"
	"sort"
)

// MaxVisible caps how many emoji rows are shown in the picker.
// Independent of mentionpicker.MaxVisible.
const MaxVisible = 5

type Model struct {
	entries  []emoji.EmojiEntry
	filtered []emoji.EmojiEntry
	frecent  []core.EmojiEntry
	query    string
	selected int
	visible  bool
	emojiCtx EmojiContext
}

// EmojiContext bundles the emoji-image rendering dependencies for
// the compose autocomplete dropdown. Mirrors the picker's version
// in shape and purpose. The Customs field is unused here because the
// entries the dropdown searches already include workspace customs
// (see emoji.BuildEntries); it's kept for shape parity with the
// other emoji-context types so all callers use the same setter
// signature.
type EmojiContext struct {
	PlaceCtx emoji.PlaceContext
	Cells    int
	Customs  map[string]string
}

// SetEmojiContext configures emoji-image rendering for the autocomplete
// dropdown. Mirrors the same setter on other UI surfaces.
func (m *Model) SetEmojiContext(ctx EmojiContext) {
	if ctx.Cells != 1 && ctx.Cells != 2 {
		ctx.Cells = 2
	}
	m.emojiCtx = ctx
}

// SetEmojiCustoms updates the customs map without changing PlaceCtx
// or Cells. Called from compose.SetEmojiCustoms when the workspace's
// custom emoji list arrives via CustomEmojisLoadedMsg.
//
// Mirrors reactionpicker.Model.SetEmojiCustoms — the picker reads
// m.emojiCtx.Customs at View() time when resolving each row's URL
// (model.go:185 calls URLForShortcode(name, m.emojiCtx.Customs)).
// Without this method the dropdown's customs map stayed at its
// startup-empty value forever, so custom emoji rows fell back to
// the placeholder glyph rendering.
func (m *Model) SetEmojiCustoms(customs map[string]string) {
	m.emojiCtx.Customs = customs
}

// HandleEmojiImageReady is a no-op hook for shape parity with other
// surfaces. The dropdown has no render cache.
func (m *Model) HandleEmojiImageReady(_ string) {}

// SetFrecentEmoji sets the frequently/recently used emoji list that
// feeds filter()'s recent tier. Mirrors
// reactionpicker.Model.SetFrecentEmoji in name and signature so both
// emoji surfaces are wired the same way from App.
//
// Entries come from core.ReactionService.LoadFrecent, i.e. the
// frecent_emoji cache table, through App — the picker does no I/O of
// its own. Passing nil or an empty slice restores tier-only ranking
// exactly (see frecent_test.go's no-op cases).
func (m *Model) SetFrecentEmoji(entries []core.EmojiEntry) {
	m.frecent = entries
	if m.visible {
		m.filter()
	}
}

func New() Model { return Model{} }

// SetEntries replaces the full entry list. If the picker is visible, the
// filtered list and selection are recomputed against the current query.
func (m *Model) SetEntries(entries []emoji.EmojiEntry) {
	m.entries = entries
	if m.visible {
		m.filter()
	}
}

func (m *Model) Open(query string) {
	m.visible = true
	m.query = query
	m.selected = 0
	m.filter()
}

func (m *Model) Close() {
	m.visible = false
	m.query = ""
	m.selected = 0
	m.filtered = nil
}

func (m *Model) IsVisible() bool { return m.visible }

func (m *Model) SetQuery(q string) {
	m.query = q
	m.selected = 0
	m.filter()
}

func (m *Model) Query() string { return m.query }

func (m *Model) Filtered() []emoji.EmojiEntry { return m.filtered }

func (m *Model) Selected() int { return m.selected }

func (m *Model) MoveUp() {
	if m.selected > 0 {
		m.selected--
	}
}

func (m *Model) MoveDown() {
	if m.selected < len(m.filtered)-1 {
		m.selected++
	}
}

// SelectedEntry returns the currently highlighted entry. ok=false if the
// filtered list is empty.
func (m *Model) SelectedEntry() (emoji.EmojiEntry, bool) {
	if len(m.filtered) == 0 {
		return emoji.EmojiEntry{}, false
	}
	if m.selected < 0 || m.selected >= len(m.filtered) {
		return emoji.EmojiEntry{}, false
	}
	return m.filtered[m.selected], true
}

// filter walks entries in input order and keeps the first MaxVisible
// matches. Callers must pass alphabetically-sorted entries
// (emoji.BuildEntries already does); the picker preserves that order.
func (m *Model) filter() {
	q := text.Fold(m.query)

	if q == "" {
		var results []emoji.EmojiEntry
		for _, e := range m.entries {
			results = append(results, e)
			if len(results) >= MaxVisible {
				break
			}
		}
		m.filtered = results
		if m.selected >= len(m.filtered) {
			m.selected = 0
			if len(m.filtered) > 0 {
				m.selected = len(m.filtered) - 1
			}
		}
		return
	}

	// frecentRank maps a name to its position in the frecent list so the
	// recent tier keeps the cache's frecency order instead of falling back
	// to the alphabetical candidate order. Lookups only: the ranking path
	// never iterates a map, which would make the sort nondeterministic.
	// Empty when no usage history exists, which leaves every rank at -1 and
	// the comparison below byte-identical to tier-only ranking.
	frecentRank := make(map[string]int, len(m.frecent))
	for i, f := range m.frecent {
		if _, dup := frecentRank[f.Name]; !dup {
			frecentRank[f.Name] = i
		}
	}

	type match struct {
		entry emoji.EmojiEntry
		tier  fuzzy.Tier
		score int
		idx   int // To preserve stable input order if tiers tie
		rank  int // position in the frecent list, or -1 when not frecent
	}
	var matches []match
	// Only m.entries are candidates. A frecent name this workspace's entry
	// list does not carry is SKIPPED rather than injected — unlike
	// reactionpicker, which injects glyph-carrying strays because
	// frecent_emoji is a global table with no team column. The dropdown
	// inserts `:name:` into a message, so a shortcode that does not resolve
	// in this workspace would be worse than absent. Pinned by
	// TestFrecent_UnknownFrecentNameIsNotInjected.
	for i, e := range m.entries {
		tier, score, ok := fuzzy.Match(e.Name, q)
		if ok {
			rank := -1
			if r, isFrecent := frecentRank[e.Name]; isFrecent {
				rank = r
			}
			matches = append(matches, match{entry: e, tier: tier, score: score, idx: i, rank: rank})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		// 1. Recent tier. It is frecent INTERSECT matches -- the loop above
		//    admits a candidate only when it matches the query -- and it
		//    orders among itself by frecency.
		if (a.rank >= 0) != (b.rank >= 0) {
			return a.rank >= 0
		}
		if a.rank >= 0 && a.rank != b.rank {
			return a.rank < b.rank
		}
		// 2. Name tier: prefix > word > squashed > substring > subsequence.
		if a.tier != b.tier {
			return a.tier < b.tier
		}
		// 3. Subsequence score; the other tiers all carry 0.
		if a.tier == fuzzy.TierSubsequence && a.score != b.score {
			return a.score > b.score // Higher score is better
		}
		// 4. Preserve input order (which is alphabetical)
		return a.idx < b.idx
	})

	var results []emoji.EmojiEntry
	for i := 0; i < len(matches) && i < MaxVisible; i++ {
		results = append(results, matches[i].entry)
	}
	m.filtered = results
	if m.selected >= len(m.filtered) {
		m.selected = 0
		if len(m.filtered) > 0 {
			m.selected = len(m.filtered) - 1
		}
	}
}

// View renders the bordered dropdown. Returns "" when not visible OR when
// there are no matches (caller already shows the textarea below).
func (m Model) View(width int) string {
	if !m.visible || len(m.filtered) == 0 {
		return ""
	}

	// Compute the widest display preview so name columns line up.
	previewWidth := 1
	for _, e := range m.filtered {
		w := lipgloss.Width(e.Display)
		if w > previewWidth {
			previewWidth = w
		}
	}

	// Image-aware emoji-as-image path: active only when the
	// process-global ImageMode is on AND a fetcher has been installed
	// via SetEmojiContext. Otherwise the legacy `e.Display` branch
	// renders (byte-identical to pre-Phase-9).
	imageOK := emoji.ImageModeActive() && m.emojiCtx.PlaceCtx.Fetcher != nil
	cells := m.emojiCtx.Cells
	if cells <= 0 {
		cells = 2
	}
	// Collect any kitty-upload callbacks Place produced into this
	// per-View local slice and fire them against imgpkg.KittyOutput
	// just before returning. Most are no-ops in steady state (the
	// messages-pane already uploaded via the shared Registry); the
	// dropdown still owns the fire to handle the case where it's the
	// first/only surface to reference a given emoji this session.
	var pendingFlushes []func(io.Writer) error

	var rows []string
	for i, e := range m.filtered {
		indicator := "  "
		nameStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)
		if i == m.selected {
			indicator = lipgloss.NewStyle().Foreground(styles.Accent).Render("▌ ")
			nameStyle = nameStyle.Bold(true)
		}

		var preview string
		if imageOK {
			if url, ok := emoji.URLForShortcode(e.Name, m.emojiCtx.Customs); ok {
				if placement, flush, ok := emoji.Place(m.emojiCtx.PlaceCtx, url, cells); ok {
					preview = placement
					if flush != nil {
						pendingFlushes = append(pendingFlushes, flush)
					}
				}
			}
		}
		if preview == "" {
			preview = e.Display
		}

		// Pad preview cell so all names start at the same column.
		pad := previewWidth - lipgloss.Width(preview)
		if pad < 0 {
			pad = 0
		}
		preview = preview + strings.Repeat(" ", pad)
		row := fmt.Sprintf("%s%s  %s", indicator, preview, nameStyle.Render(":"+e.Name+":"))
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")
	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Primary).
		Background(styles.SurfaceDark).
		Width(width - 2).
		Render(content)

	// Fire any kitty image upload callbacks the per-row Place calls
	// produced. Done here (inside View) so the autocomplete dropdown
	// owns kitty uploads independently of any other surface.
	for _, fl := range pendingFlushes {
		_ = fl(imgpkg.KittyOutput)
	}
	return box
}
