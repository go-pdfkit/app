package main

import (
	"testing"

	"github.com/go-gfx/gfx/raster"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
	"github.com/go-pdfkit/render"
	"github.com/go-widgets/toolkit"
)

// tally counts what one gesture costs: how many times the whole document was
// written out and read back, how many bytes that was, and how many pages were
// rendered.
//
// ⛔ COUNTS, not times. This machine runs ~25 sessions at once and its load
// average is regularly past 30; a stopwatch here would measure the neighbours.
// Allocation and call counts are exact whatever else is running, and the
// defect this is here to hold shut is a COUNT: a navigation that rewrites the
// file.
type tally struct {
	writes  int
	written int
	reads   int
	renders int
}

func count(t *testing.T) *tally {
	t.Helper()
	c := &tally{}
	wasWrite, wasOpen, wasDraw := docBytes, openBytes, drawPage
	t.Cleanup(func() { docBytes, openBytes, drawPage = wasWrite, wasOpen, wasDraw })
	docBytes = func(d *ops.Doc) ([]byte, error) {
		out, err := wasWrite(d)
		c.writes++
		c.written += len(out)
		return out, err
	}
	openBytes = func(data []byte, pw string) (*reader.Document, error) {
		c.reads++
		return wasOpen(data, pw)
	}
	drawPage = func(src *reader.Document, i int, o render.Options) (*raster.Image, error) {
		c.renders++
		return wasDraw(src, i, o)
	}
	return c
}

// TestGoingToAPageDoesNotRewriteTheDocument is the measurement behind the
// report "l'application lag".
//
// ⛔ Going to a page CHANGES NOTHING ABOUT THE DOCUMENT. Before this, every
// press on a minipage ran refresh, and refresh writes the whole file out,
// parses it back, throws away every page picture and re-renders the visible
// ones -- for a gesture whose entire effect is that a different number is lit.
func TestGoingToAPageDoesNotRewriteTheDocument(t *testing.T) {
	s, _ := opened(t, 40)
	s.draw(buffer())

	c := count(t)
	s.goTo(20)
	s.draw(buffer())

	if c.writes != 0 || c.reads != 0 {
		t.Errorf("going to page 20 wrote the document %d times (%d bytes), parsed it back %d times and re-rendered %d pages",
			c.writes, c.written, c.reads, c.renders)
	}
	if s.at != 20 {
		t.Fatalf("the workbench is on page %d", s.at)
	}
}

// TestGoingToAPageRebuildsABandThatIsNotTheViewer is the other half of the
// cheap path: it is only right while the band is holding the VIEWER. A reading
// or a filled-in form is built FROM the document, so a band holding one has to
// be remade, and going to a page there pays the full price on purpose.
func TestGoingToAPageRebuildsABandThatIsNotTheViewer(t *testing.T) {
	s, _ := opened(t, 6)
	s.tools.reading.Set("text")
	s.refresh()
	s.draw(buffer())
	if _, viewing := s.shown.(*toolkit.PagedView); viewing {
		t.Fatal("precondition: the band should be holding a reading, not the viewer")
	}

	c := count(t)
	s.goTo(4)
	if c.writes == 0 {
		t.Error("a band that is not the viewer was not rebuilt")
	}
	if s.at != 4 {
		t.Errorf("the workbench is on page %d", s.at)
	}
}

// TestDraggingTheDividerCostsNothingPerFrame is the measurement behind the
// second report: "resizing the bars is very slow, I have the impression the
// whole tab lags".
//
// ⛔ THE FRAMES ARE THE POINT. dragHandle presses, moves and releases back to
// back with no frame in between, which is how the gesture defect before this
// one hid: what a drag costs is what it costs ON EVERY FRAME while the pointer
// is down, and a test with no frames in it measures none of them.
func TestDraggingTheDividerCostsNothingPerFrame(t *testing.T) {
	s, _ := opened(t, 40)
	s.draw(buffer())
	sp := splitterIn(s.view)
	if sp == nil {
		t.Fatal("there is no divider beside the page")
	}
	r, ok := sp.HandleRect(0)
	if !ok {
		t.Fatalf("the divider has no rectangle to grab: %+v", r)
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	s.handleClick(x, y)

	c := count(t)
	const frames = 20
	for i := 1; i <= frames; i++ {
		s.handleMove(x+2*i, y)
		s.tick(0.016)
		s.draw(buffer())
	}
	s.handleRelease(x+2*frames, y)

	if c.writes != 0 || c.reads != 0 {
		t.Errorf("%d frames of dragging the divider wrote the document %d times (%d bytes) "+
			"and parsed it back %d times", frames, c.writes, c.written, c.reads)
	}
	// ⛔ And no page is re-rendered WHILE the divider is moving. The pictures
	// are remade once the reader lets go, at the width they settled on —
	// re-rendering forty pages at each of twenty intermediate widths is the
	// whole of what "very slow" would be.
	if c.renders != 0 {
		t.Errorf("%d frames of dragging re-rendered %d pages before the reader let go",
			frames, c.renders)
	}
	if s.railWidth.Get() <= railLogicalW {
		t.Fatalf("the drag moved nothing: the rail is %d wide", s.railWidth.Get())
	}
}

// TestAVerbStillRewritesTheDocument is the control for the test above: it
// would pass if goTo simply stopped working, so the cost that MUST still be
// paid is asserted too. A verb edits the file, and what is on the screen after
// one has to be what would come out of Save.
func TestAVerbStillRewritesTheDocument(t *testing.T) {
	s, _ := opened(t, 6)
	s.draw(buffer())

	c := count(t)
	s.rotate()
	s.draw(buffer())

	if c.writes == 0 || c.reads == 0 {
		t.Errorf("rotating wrote the document %d times and parsed it back %d times; a verb must do both",
			c.writes, c.reads)
	}
}
