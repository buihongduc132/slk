// internal/ui/sixel_zoom_placement_test.go
//
// B51. App.collectSixelPlacements derives every placement rectangle from
// a single panelLayout.Compute, so it is sound BY CONSTRUCTION -- but
// nothing asserted that a placement's geometry actually TRACKS a.zoomed.
// Zoom's whole effect on the vertical axis is panelLayout.statusRows(),
// which returns 0 while zoomed and 1 otherwise. That one row flows:
//
//	statusRows() -> frame.ContentHeight -> bounds.H -> rect.H
//	  -> the window model's render height, hence its window-row math
//	  -> `bottom` in absoluteWindowSixelPlacements
//
// A regression that stopped propagating zoom into that height failed no
// existing test: sixelpaint_test.go calls collectSixelPlacements twice
// but never across a zoom transition, so it would have surfaced only as
// misdrawn images at runtime.
//
// Both tests below assert a RELATIONSHIP against `reclaimed`, never an
// observed row number:
//
//	reclaimed := a.height - unzoomedFrame.ContentHeight
//
// -- how many bottom rows the status bar owns while unzoomed, read back
// from the same Compute the production code calls. Zoom's contract is
// that the content pane gets exactly those rows. Deriving it from the
// UNZOOMED leg is deliberate: that leg is what a zoom-blind statusRows()
// leaves untouched, so the expectation cannot drift along with the bug.
package ui

import (
	"testing"

	imgpkg "github.com/gammons/slk/internal/image"
)

// sixelOverflowApp builds a sixel-bearing App whose messages pane
// OVERFLOWS, which is what makes zoom's extra row observable.
//
// The cache/ImageContext staging is setupTwoWindowSixelImages's
// (sixelpaint_test.go), reused rather than restaged: the primed
// rendition there (320x320 target, 8x16 cells, MaxRows 20) is what makes
// Fetcher.Cached hit at all, and a different MaxRows silently yields no
// placement instead of a smaller one.
//
// It then RE-SEEDS the messages with filler and the image last. With the
// single image message that fixture seeds, the pane does not overflow,
// the viewport sits at offset 0, and the placement lands on the SAME row
// zoomed and unzoomed -- measured: no observable difference. Overflow
// bottom-anchors the viewport (yOffset = totalLines - msgAreaHeight), so
// the trailing image's window row moves with the pane's content height,
// which is precisely the quantity zoom changes.
//
// withWindowSize, not withSize: the sixel path needs the real resize
// path, which propagates dimensions into the sub-models and sets
// forceSixelRepaint.
func sixelOverflowApp(t *testing.T, w, h int) *App {
	t.Helper()
	a := newTestApp(t, withWindowSize(w, h))
	a.imgProtocol = imgpkg.ProtoSixel
	a.sixelFrames = imgpkg.NewSixelFrameStore()
	setupTwoWindowSixelImages(t, a)
	_, _, _, img := imageBearingMessage(t)
	msgs := append(testMessageItems(40), img)
	for _, m := range a.allWinModels() {
		m.SetMessages(msgs)
	}
	return a
}

// framePlacements drives one real View and returns the placements it
// published. Going through View, rather than calling
// collectSixelPlacements against a hand-built rect as
// TestCollectSixelPlacements_TwoWindowsFocusedAndUnfocused does, is
// deliberate: production renders window models at rect.H-2
// (renderUnfocusedWindow), so a fixture that renders at rect.H leaves
// the model's own visibility gate 2 rows looser than the real one. Only
// View keeps every height in the chain honest.
func framePlacements(t *testing.T, a *App) []imgpkg.SixelPlacement {
	t.Helper()
	v := a.View()
	f, ok := a.sixelFrames.Take(frameIDFromTitle(t, v.WindowTitle))
	if !ok {
		t.Fatal("View published no sixel frame")
	}
	return f.Placements
}

// TestSixelPlacement_ZoomedPlacementGainsTheStatusRow: the pane is
// bottom-anchored, so growing its content area by the status row moves
// the image it ends on down by exactly that many rows. Asserted as a
// translation, with the footprint pinned unchanged so a resize cannot
// masquerade as the shift.
func TestSixelPlacement_ZoomedPlacementGainsTheStatusRow(t *testing.T) {
	for _, size := range []struct{ w, h int }{{120, 35}, {200, 44}} {
		a := sixelOverflowApp(t, size.w, size.h)

		unzoomed := a.computeFrame()
		before := a.View().Content
		up := framePlacements(t, a)

		reclaimed := a.height - unzoomed.ContentHeight
		if reclaimed <= 0 {
			t.Fatalf("%dx%d: the unzoomed layout reserves %d rows for the status bar; with nothing "+
				"reserved there is nothing for zoom to reclaim and this test is vacuous", size.w, size.h, reclaimed)
		}
		if len(up) != 1 {
			t.Fatalf("%dx%d: unzoomed frame published %d placements, want exactly 1 -- the fixture must "+
				"put one paintable sixel in the overflowing pane", size.w, size.h, len(up))
		}

		mustEnterZoom(t, a, before)
		zoomedFrame := a.computeFrame()
		zp := framePlacements(t, a)
		if len(zp) != 1 {
			t.Fatalf("%dx%d: zoomed frame published %d placements, want exactly 1", size.w, size.h, len(zp))
		}

		if got := zoomedFrame.ContentHeight - unzoomed.ContentHeight; got != reclaimed {
			t.Errorf("%dx%d: zoom gave the content area %d extra rows, want %d (the rows the status bar "+
				"owns while unzoomed)", size.w, size.h, got, reclaimed)
		}
		if got := zp[0].Row - up[0].Row; got != reclaimed {
			t.Errorf("%dx%d: the placement moved %d rows on zoom, want %d -- the bottom-anchored pane grew "+
				"by the status row, so the trailing image must move down by exactly that much "+
				"(unzoomed Row=%d, zoomed Row=%d)", size.w, size.h, got, reclaimed, up[0].Row, zp[0].Row)
		}
		if zp[0].Rows != up[0].Rows || zp[0].Cols != up[0].Cols {
			t.Errorf("%dx%d: zoom resized the footprint %dx%d -> %dx%d; the row gain must be a pure "+
				"translation", size.w, size.h, up[0].Cols, up[0].Rows, zp[0].Cols, zp[0].Rows)
		}
	}
}

// TestSixelPlacement_ZoomLowersTheHeightAtWhichAnImageFits is the second,
// independent consequence of the same row: a taller content area is a
// taller paintable band, so zoom must make an image fit at a terminal
// height where the unzoomed layout rejects it. The two thresholds are
// SEARCHED, not written down, so the assertion stays a relationship.
func TestSixelPlacement_ZoomLowersTheHeightAtWhichAnImageFits(t *testing.T) {
	const w, lo, hi = 120, 24, 40
	minUnzoomed, minZoomed, reclaimed := 0, 0, -1

	for h := lo; h <= hi && (minUnzoomed == 0 || minZoomed == 0); h++ {
		a := sixelOverflowApp(t, w, h)
		got := a.height - a.computeFrame().ContentHeight
		if reclaimed < 0 {
			reclaimed = got
		} else if got != reclaimed {
			t.Fatalf("h=%d: the unzoomed status bar reserves %d rows, %d at the other heights; this test "+
				"assumes one rule for every height", h, got, reclaimed)
		}

		nUnzoomed := len(framePlacements(t, a))
		updateAndRender(t, a, keyPress('z'))
		if !a.zoomed {
			t.Fatalf("h=%d: 'z' did not enter zoom", h)
		}
		nZoomed := len(framePlacements(t, a))

		if nZoomed < nUnzoomed {
			t.Errorf("h=%d: zoom LOST a placement (%d -> %d); a taller content area cannot reject an "+
				"image the shorter one accepted", h, nUnzoomed, nZoomed)
		}
		if minZoomed == 0 && nZoomed > 0 {
			minZoomed = h
		}
		if minUnzoomed == 0 && nUnzoomed > 0 {
			minUnzoomed = h
		}
	}

	if reclaimed <= 0 {
		t.Fatalf("the unzoomed layout reserves %d status rows; nothing for zoom to reclaim", reclaimed)
	}
	if minUnzoomed == 0 || minZoomed == 0 {
		t.Fatalf("no first-fit height in [%d,%d] (zoomed=%d unzoomed=%d): the fixture's image no longer "+
			"fits anywhere in range, so the thresholds cannot be compared", lo, hi, minZoomed, minUnzoomed)
	}
	if got := minUnzoomed - minZoomed; got != reclaimed {
		t.Errorf("the image first fits at height %d zoomed and %d unzoomed, a %d-row difference; want %d "+
			"-- zoom frees exactly the status bar's rows, so it must lower the fitting threshold by that "+
			"much", minZoomed, minUnzoomed, got, reclaimed)
	}
}
