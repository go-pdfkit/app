package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/ops"
	"github.com/go-pdfkit/reader"
)

// gridPDF is a page whose words line up into a two-column grid, drawn the way
// a producer draws one: text set at places, with no table anywhere in the file.
//
// ⛔ THAT IS THE WHOLE POINT OF THE FIXTURE. If it declared a table, it would
// be testing something a PDF cannot hold. The columns are at x 40 and x 160,
// the rows 20 points apart, and whether those places line up is the inference
// extract is being asked to make.
func gridPDF(t *testing.T) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	widths := make(reader.Array, 0, 224)
	for i := 32; i < 256; i++ {
		widths = append(widths, reader.Integer(500))
	}
	font := w.Add(reader.Dict{
		"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
		"BaseFont": reader.Name("Helvetica"), "FirstChar": reader.Integer(32),
		"LastChar": reader.Integer(255), "Widths": widths,
		"Encoding": reader.Name("WinAnsiEncoding"),
	})
	var body strings.Builder
	rows := [][2]string{
		{"Item", "Cost"},
		{"Paper", "12"},
		{"Ink", "34"},
		{"Press", "56"},
	}
	y := 300.0
	for _, row := range rows {
		fmt.Fprintf(&body, "BT /F1 10 Tf 40 %.0f Td (%s) Tj ET\n", y, row[0])
		fmt.Fprintf(&body, "BT /F1 10 Tf 160 %.0f Td (%s) Tj ET\n", y, row[1])
		y -= 20
	}
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"Contents":  w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(body.String())}),
		"Resources": reader.Dict{"Font": reader.Dict{"F1": font}},
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(300), reader.Integer(400)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ⛔⛔ THREE OF BENTOPDF'S TOOLS FOR NOTHING, which is why this is worth a
// test of its own. They were recorded against `go-pdfkit/document`, which
// costs about 33.5 MB in a wasm binary in either direction because the package
// imports html2pdf and so the browser engine. `extract.Tables` answers the
// same question and has been linked since the Read tab could read text.
func TestTheGridsOfAPageAreShown(t *testing.T) {
	s := shown(t, gridPDF(t))
	s.readTables()
	if s.tools.reading.Get() != readingTables {
		t.Fatalf("the reading is %q", s.tools.reading.Get())
	}
	// The view is built from the document as it would be saved, like the
	// picture of it.
	w := s.tablesView(s.src)
	if w == nil {
		t.Fatal("the grids view is nothing")
	}
	tables, _, ok := s.pageTables()
	if !ok {
		t.Fatal("the page's grids could not be read")
	}
	if len(tables) != 1 {
		t.Fatalf("the page lines up into %d grid(s), want 1", len(tables))
	}
	if got := len(tables[0].Rows); got != 4 {
		t.Errorf("the grid has %d rows, want 4: %v", got, tables[0].Rows)
	}
	if got := columnsOf(tables[0]); got != 2 {
		t.Errorf("the grid has %d columns, want 2: %v", got, tables[0].Rows)
	}
}

// A page whose words do not line up says so, rather than showing an empty
// heading a reader cannot tell from a failure.
func TestAPageWithNoGridSaysSo(t *testing.T) {
	s := shown(t, perGlyphPDF(t))
	w := s.tablesView(s.src)
	if w == nil {
		t.Fatal("the grids view is nothing")
	}
	tables, _, ok := s.pageTables()
	if !ok {
		t.Fatal("the page could not be read")
	}
	if len(tables) != 0 {
		t.Errorf("one line of prose lined up into %d grid(s)", len(tables))
	}
}

// ⛔ THE CSV IS READ BACK WITH A CSV READER, not compared as a string. The
// point of handing the work to encoding/csv is that it quotes whatever needs
// quoting; a test asserting the exact bytes would pass for a writer that
// quoted nothing and fail for one that quoted correctly.
func TestTheGridsAreHandedOverAsCSV(t *testing.T) {
	s, h := openedGrid(t)
	s.tablesAsCSV()
	if h.as == "" {
		t.Fatal("nothing was handed over")
	}
	if !strings.HasSuffix(h.as, ".csv") {
		t.Errorf("the file was called %q", h.as)
	}
	rows, err := csv.NewReader(strings.NewReader(string(h.saved))).ReadAll()
	if err != nil {
		t.Fatalf("what was handed over is not CSV: %v", err)
	}
	want := [][]string{
		{"Item", "Cost"}, {"Paper", "12"}, {"Ink", "34"}, {"Press", "56"},
	}
	if len(rows) != len(want) {
		t.Fatalf("%d rows were handed over, want %d: %v", len(rows), len(want), rows)
	}
	for i, row := range want {
		if len(rows[i]) != len(row) || rows[i][0] != row[0] || rows[i][1] != row[1] {
			t.Errorf("row %d is %v, want %v", i, rows[i], row)
		}
	}
}

// ⛔⛔ THE JSON CARRIES THE POSITIONS, and without them it would be the text
// file beside it under another name. What makes a page worth having as data is
// that every piece says where it was.
func TestThePageIsHandedOverAsData(t *testing.T) {
	s, h := openedGrid(t)
	s.pageAsJSON()
	if !strings.HasSuffix(h.as, ".json") {
		t.Fatalf("the file was called %q", h.as)
	}
	var got struct {
		Page int `json:"page"`
		Text []struct {
			Text string  `json:"text"`
			X    float64 `json:"x"`
			Y    float64 `json:"y"`
		} `json:"text"`
		Grids []struct {
			Rows [][]string `json:"rows"`
			X    float64    `json:"x"`
		} `json:"grids"`
	}
	if err := json.Unmarshal(h.saved, &got); err != nil {
		t.Fatalf("what was handed over is not JSON: %v", err)
	}
	if got.Page != 1 {
		t.Errorf("the data says page %d", got.Page)
	}
	if len(got.Text) == 0 {
		t.Fatal("the data carries no text")
	}
	placed := false
	for _, p := range got.Text {
		if p.X > 0 && p.Y > 0 {
			placed = true
		}
	}
	if !placed {
		t.Error("every piece of text is at the origin, so the positions were lost")
	}
	if len(got.Grids) != 1 || len(got.Grids[0].Rows) != 4 {
		t.Errorf("the data carries %d grid(s)", len(got.Grids))
	}
}

// Both hand-overs need a document, and say so.
func TestTheGridVerbsNeedADocument(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func(*state)
	}{
		{"the grids", (*state).tablesAsCSV},
		{"the data", (*state).pageAsJSON},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newState(surfaceW, surfaceH, &fakeHost{})
			c.run(s)
			if !strings.Contains(s.note, "open a document first") {
				t.Errorf("the status line says %q", s.note)
			}
		})
	}
}

// ⛔⛔ THE SEPARATOR BETWEEN TWO GRIDS HAS TO BE ONE A READER CAN SEE, and the
// obvious choice cannot be. A blank line is what `pdfops tables -as csv`
// writes, with a comment saying "so a reader can see where each ended" — and
// `encoding/csv` SKIPS blank lines on read, so the file comes back as one grid
// with the next one's rows stuck to it and nothing saying where the join is.
// This test is what found that (go-pdfkit/ops#71): it read its own output back
// with a CSV reader rather than comparing bytes, and the separator had
// vanished.
//
// The fixture is checked for TWO grids before anything else, because one that
// lined up into a single grid would pass a test about separators by never
// needing one.
func TestTwoGridsAreSeparatedBySomethingAReaderCanSee(t *testing.T) {
	s, h := openedTwoGrids(t)
	tables, _, ok := s.pageTables()
	if !ok {
		t.Fatal("the page's grids could not be read")
	}
	if len(tables) != 2 {
		t.Fatalf("the fixture lines up into %d grid(s), want 2", len(tables))
	}
	s.tablesAsCSV()
	// FieldsPerRecord off, because the marker row is one cell and the grids
	// are two: a strict reader refuses any file that says where a table ends.
	r := csv.NewReader(strings.NewReader(string(h.saved)))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("what was handed over is not CSV: %v", err)
	}
	markers := 0
	for _, row := range rows {
		if len(row) == 1 && strings.HasPrefix(row[0], "grid ") {
			markers++
		}
	}
	if markers != 2 {
		t.Errorf("%d marker row(s) name the grids, want 2: %v", markers, rows)
	}
	// And the rows of both grids are still there, in order.
	flat := fmt.Sprint(rows)
	for _, want := range []string{"Paper 12", "June 80"} {
		if !strings.Contains(flat, want) {
			t.Errorf("the file lost %q: %v", want, rows)
		}
	}
}

// One grid gets no marker: a file that says "grid 1 of 1" is a file explaining
// something nobody asked about.
func TestOneGridNeedsNoMarker(t *testing.T) {
	s, h := openedGrid(t)
	s.tablesAsCSV()
	if strings.Contains(string(h.saved), "grid ") {
		t.Errorf("a page with one grid was marked up: %q", h.saved)
	}
}

// ⛔ AND A FAILURE TO WRITE IS TOLD, NOT HANDED OVER. This is the branch the
// app already cares about for pictures — encodeImage is a variable for exactly
// this reason — because the alternative is somebody receiving a truncated file
// and finding out later.
func TestAFailureToWriteIsReportedRatherThanHandedOver(t *testing.T) {
	t.Run("the grids", func(t *testing.T) {
		s, h := openedGrid(t)
		was := writeCSV
		writeCSV = func([]extract.Table) ([]byte, error) { return nil, errWrite }
		t.Cleanup(func() { writeCSV = was })
		s.tablesAsCSV()
		if h.as != "" {
			t.Errorf("%q was handed over anyway", h.as)
		}
		if !strings.Contains(s.note, "cannot be written") {
			t.Errorf("the status line says %q", s.note)
		}
	})
	t.Run("the data", func(t *testing.T) {
		s, h := openedGrid(t)
		was := marshalJSON
		marshalJSON = func(any) ([]byte, error) { return nil, errWrite }
		t.Cleanup(func() { marshalJSON = was })
		s.pageAsJSON()
		if h.as != "" {
			t.Errorf("%q was handed over anyway", h.as)
		}
		if !strings.Contains(s.note, "cannot be written") {
			t.Errorf("the status line says %q", s.note)
		}
	})
}

// A page that refuses to be read is said so about, in the view and in both
// hand-overs.
func TestAPageThatCannotBeReadIsSaidSoAbout(t *testing.T) {
	armTables := func(t *testing.T) {
		was := extractTables
		extractTables = func(*reader.Document, int) ([]extract.Table, error) {
			return nil, errWrite
		}
		t.Cleanup(func() { extractTables = was })
	}
	t.Run("the view", func(t *testing.T) {
		s := shown(t, gridPDF(t))
		armTables(t)
		if w := s.tablesView(s.src); w == nil {
			t.Fatal("the view is nothing")
		}
	})
	t.Run("the hand-over", func(t *testing.T) {
		s, h := openedGrid(t)
		armTables(t)
		s.tablesAsCSV()
		if h.as != "" {
			t.Errorf("%q was handed over anyway", h.as)
		}
		if !strings.Contains(s.note, "cannot be read") {
			t.Errorf("the status line says %q", s.note)
		}
	})
	t.Run("the text of the data", func(t *testing.T) {
		s, h := openedGrid(t)
		was := extractRuns
		extractRuns = func(*reader.Document, int) ([]extract.Run, error) {
			return nil, errWrite
		}
		t.Cleanup(func() { extractRuns = was })
		s.pageAsJSON()
		if h.as != "" {
			t.Errorf("%q was handed over anyway", h.as)
		}
		if !strings.Contains(s.note, "cannot be read") {
			t.Errorf("the status line says %q", s.note)
		}
	})
}

// errWrite is the failure the seams above are made to report.
var errWrite = fmt.Errorf("this page refuses")

// ⛔ A document that cannot be written back cannot be read for its grids
// either, and the reason is the same one every reading on this tab gives: what
// is listed is read out of the document written and read back, so that it is
// what would come out of Save. If that write fails there is nothing to read,
// and the verb has to say so rather than hand over an empty file. `docBytes`
// is the seam the picture hand-over already uses for this.
func TestGridsOfADocumentThatCannotBeWrittenBack(t *testing.T) {
	s, h := openedGrid(t)
	was := docBytes
	t.Cleanup(func() { docBytes = was })
	docBytes = func(*ops.Doc) ([]byte, error) { return nil, errWrite }
	s.tablesAsCSV()
	if h.as != "" {
		t.Errorf("%q was handed over anyway", h.as)
	}
	if !strings.Contains(s.note, "cannot be written") {
		t.Errorf("the status line says %q", s.note)
	}
}

// openedTwoGrids is a page with two blocks of aligned words, far enough apart
// to be two grids.
func openedTwoGrids(t *testing.T) (*state, *fakeHost) {
	t.Helper()
	h := &fakeHost{name: "grids.pdf", file: twoGridsPDF(t)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	s.draw(buffer())
	return s, h
}

// twoGridsPDF draws two two-column blocks, one near the top and one near the
// foot, with a wide band of nothing between them.
func twoGridsPDF(t *testing.T) []byte {
	t.Helper()
	w := reader.NewWriter("1.7")
	pagesRef := w.Reserve()
	widths := make(reader.Array, 0, 224)
	for i := 32; i < 256; i++ {
		widths = append(widths, reader.Integer(500))
	}
	font := w.Add(reader.Dict{
		"Type": reader.Name("Font"), "Subtype": reader.Name("Type1"),
		"BaseFont": reader.Name("Helvetica"), "FirstChar": reader.Integer(32),
		"LastChar": reader.Integer(255), "Widths": widths,
		"Encoding": reader.Name("WinAnsiEncoding"),
	})
	var body strings.Builder
	block := func(top float64, rows [][2]string) {
		y := top
		for _, row := range rows {
			fmt.Fprintf(&body, "BT /F1 10 Tf 40 %.0f Td (%s) Tj ET\n", y, row[0])
			fmt.Fprintf(&body, "BT /F1 10 Tf 160 %.0f Td (%s) Tj ET\n", y, row[1])
			y -= 20
		}
	}
	block(700, [][2]string{{"Item", "Cost"}, {"Paper", "12"}, {"Ink", "34"}})
	block(200, [][2]string{{"Month", "Total"}, {"May", "90"}, {"June", "80"}})
	page := w.Add(reader.Dict{
		"Type": reader.Name("Page"), "Parent": pagesRef,
		"Contents":  w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(body.String())}),
		"Resources": reader.Dict{"Font": reader.Dict{"F1": font}},
	})
	w.Put(pagesRef, reader.Dict{"Type": reader.Name("Pages"),
		"Kids": reader.Array{page}, "Count": reader.Integer(1),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0),
			reader.Integer(400), reader.Integer(800)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pagesRef})
	out, err := w.Finish(reader.Dict{"Root": root})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// openedGrid opens the grid fixture and hands back the host, so what was
// handed over can be read back.
func openedGrid(t *testing.T) (*state, *fakeHost) {
	t.Helper()
	h := &fakeHost{name: "grid.pdf", file: gridPDF(t)}
	s := newState(surfaceW, surfaceH, h)
	s.open()
	if s.doc == nil {
		t.Fatalf("the document did not open: %q", s.note)
	}
	s.draw(buffer())
	return s, h
}

// A row of empty cells shows the gaps as gaps rather than as nothing at all,
// so a reader can see which column was blank.
func TestAnEmptyCellIsVisibleInTheView(t *testing.T) {
	if got := joinCells([]string{"a", "", "c"}); !strings.Contains(got, "—") {
		t.Errorf("a row with an empty cell reads %q", got)
	}
	// ⛔ And the separator is not a comma: a row joined with commas reads as a
	// CSV row and is not one, so a cell holding a comma would look like two.
	if strings.Contains(joinCells([]string{"a,b", "c"}), ", ") {
		t.Error("the view joins cells with something a cell can contain")
	}
}

// One grid is "the grid on this page"; several are numbered.
func TestAGridIsNamedForHowManyThereAre(t *testing.T) {
	if got := tableName(0, 1); !strings.Contains(got, "the grid on this page") {
		t.Errorf("one grid is called %q", got)
	}
	if got := tableName(1, 3); !strings.Contains(got, "2 of 3") {
		t.Errorf("the second of three is called %q", got)
	}
}

// An empty grid has no columns, and asking does not panic.
func TestAGridWithNoRowsHasNoColumns(t *testing.T) {
	if got := columnsOf(extract.Table{}); got != 0 {
		t.Errorf("a grid with no rows has %d columns", got)
	}
}
