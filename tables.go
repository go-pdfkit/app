// The grids a page lines its words up into, shown and handed over.
//
// ⛔⛔ THROUGH `extract`, WHICH THE WORKBENCH ALREADY CARRIES — and that is the
// point of this file existing at all. Three of BentoPDF's tools (Extract
// Tables, PDF to CSV, PDF to JSON) were recorded against `go-pdfkit/document`
// and `pdfops tables`, and `document` costs about 33.5 MB in a wasm binary
// whichever direction it is used in, because the package imports html2pdf and
// so the browser engine behind it. Measured 2026-11-08: 3.50 MB for ooxml's
// writer alone, 37.05 MB once `document.Parse` is named.
//
// `extract.Tables` answers the same question for nothing: it is already linked,
// because the Read tab has read the text of a page since the beginning. Anchor
// on the repository a verdict happens to cite and three free rows look like
// thirty-three megabytes.
//
// ⛔ And a table on a PDF page is an INFERENCE. extract says so itself: "A PDF
// page does not contain a table. It contains text put at places, and a table is
// what a reader sees when those places line up." So what is shown and handed
// over is what the alignment said, and the wording here never promises the
// author declared it.

package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"

	"github.com/go-pdfkit/extract"
	"github.com/go-pdfkit/reader"
	"github.com/go-widgets/toolkit"
)

// readingTables is the reading that shows the grids instead of the page.
const readingTables = "tables"

// readTables shows the page's tables, or puts the page back.
func (s *state) readTables() { s.read(readingTables) }

// tablesView is every grid the page's words line up into.
func (s *state) tablesView(src *reader.Document) toolkit.Widget {
	tables, err := extractTables(src, s.at)
	if err != nil {
		return toolkit.NewLabel("the tables of this page cannot be read: " + err.Error())
	}
	if len(tables) == 0 {
		return toolkit.NewLabel("nothing on this page lines up into a grid")
	}
	col := newColumn()
	for i, t := range tables {
		col.add(toolkit.NewLabel(fmt.Sprintf("%s — %d row(s) of %d",
			tableName(i, len(tables)), len(t.Rows), columnsOf(t))), lineH)
		for _, row := range t.Rows {
			col.add(toolkit.NewLabel("    "+joinCells(row)), lineH)
		}
		col.add(toolkit.NewLabel(""), lineH)
	}
	return col.scrollerOf(s.pageW() - 2*margin)
}

// tableName is what to call one grid of several.
func tableName(i, of int) string {
	if of == 1 {
		return "the grid on this page"
	}
	return fmt.Sprintf("grid %d of %d", i+1, of)
}

// columnsOf is how wide a table is. Every row holds the same number of cells,
// so the first row answers for all of them.
func columnsOf(t extract.Table) int {
	if len(t.Rows) == 0 {
		return 0
	}
	return len(t.Rows[0])
}

// joinCells shows one row, with the empty cells visible as the gaps they are.
//
// ⛔ A separator a cell cannot contain. A row joined with a comma reads as a
// CSV row and is not one — a cell holding a comma would look like two cells,
// and the person reading it would have no way to tell. The CSV handed over by
// the verb below quotes properly; this is for looking at.
func joinCells(row []string) string {
	var b bytes.Buffer
	for i, cell := range row {
		if i > 0 {
			b.WriteString("  │  ")
		}
		if cell == "" {
			b.WriteString("—")
			continue
		}
		b.WriteString(cell)
	}
	return b.String()
}

// tablesAsCSV hands the page's grids over as comma-separated values.
//
// ⛔ ONE FILE, WITH A BLANK LINE BETWEEN GRIDS. A page may line up into
// several, and CSV has no way to say "a second table starts here" — so the
// choice is one file per grid or one file with a separator, and a reader who
// asked for "this page as CSV" gets one file. encoding/csv quotes whatever
// needs quoting, which is the reason not to join the cells by hand.
func (s *state) tablesAsCSV() {
	tables, _, ok := s.pageTables()
	if !ok {
		return
	}
	out, err := writeCSV(tables)
	if err != nil {
		s.fail("the grids cannot be written: " + err.Error())
		return
	}
	s.handOver(fmt.Sprintf("page%03d.csv", s.at), out)
}

// writeCSV is a variable for the reason encodeImage beside it is one: the
// branch where writing fails is the branch that decides whether somebody is
// handed a truncated file or told, and it is the only part of this that a test
// cannot otherwise reach — a csv.Writer over a bytes.Buffer has nothing to
// fail against.
//
// ⛔ ONE ERROR CHECK, AFTER THE FLUSH. csv.Writer remembers the first failure
// and Error() hands it back, so checking each Write as well would be several
// unreachable branches saying the same thing.
//
// ⛔⛔ AND THE SEPARATOR IS A ROW, NOT A BLANK LINE. A blank line is the
// obvious choice and it does not work: `encoding/csv` SKIPS blank lines when
// reading, so a file written that way reads back as one grid with the rows of
// the next stuck to it, and nothing says where the join is. `pdfops tables -as
// csv` writes `w.Write(nil)` between grids with a comment saying "so a reader
// can see where each ended" — it cannot, and a test here reading its own
// output back is what showed it (go-pdfkit/ops#71).
//
// RFC 4180 has no way to say "another table starts here", so the choice is a
// row that says it or a file per grid. A row keeps "this page as CSV" to one
// file and is visible to every reader, including a spreadsheet.
var writeCSV = func(tables []extract.Table) ([]byte, error) {
	var out bytes.Buffer
	w := csv.NewWriter(&out)
	for i, t := range tables {
		if len(tables) > 1 {
			w.Write([]string{fmt.Sprintf("grid %d of %d", i+1, len(tables))}) //nolint:errcheck // Error() below reports it
		}
		for _, row := range t.Rows {
			w.Write(row) //nolint:errcheck // Error() below reports it
		}
	}
	w.Flush()
	return out.Bytes(), w.Error()
}

// pageAsJSON is the page's words and grids, with where each of them sits.
//
// ⛔ WITH THE POSITIONS, because without them this is just the text file next
// to it under another name. What makes a page worth having as JSON is that
// every piece says where it was — which is what lets somebody else's program
// find the total in the bottom right corner without guessing.
func (s *state) pageAsJSON() {
	tables, src, ok := s.pageTables()
	if !ok {
		return
	}
	runs, err := extractRuns(src, s.at)
	if err != nil {
		s.fail("this page cannot be read: " + err.Error())
		return
	}
	type piece struct {
		Text    string  `json:"text"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
		Width   float64 `json:"width"`
		Size    float64 `json:"size"`
		Font    string  `json:"font,omitempty"`
		Unknown bool    `json:"unreadable,omitempty"`
	}
	// ⛔ One tag per field. `X, Y, Width, Height float64 \`json:"x,y,width,height"\``
	// compiles, and gives all four the SAME name — four keys called "x" in one
	// object, which is a document no JSON reader agrees about.
	type grid struct {
		Rows   [][]string `json:"rows"`
		X      float64    `json:"x"`
		Y      float64    `json:"y"`
		Width  float64    `json:"width"`
		Height float64    `json:"height"`
	}
	page := struct {
		Page   int     `json:"page"`
		Pieces []piece `json:"text"`
		Grids  []grid  `json:"grids"`
	}{Page: s.at, Pieces: []piece{}, Grids: []grid{}}
	for _, r := range runs {
		page.Pieces = append(page.Pieces, piece{
			Text: r.Text, X: r.X, Y: r.Y, Width: r.Width, Size: r.Size,
			Font: r.Font, Unknown: r.Unreadable,
		})
	}
	for _, t := range tables {
		page.Grids = append(page.Grids, grid{
			Rows: t.Rows, X: t.X, Y: t.Y, Width: t.Width, Height: t.Height,
		})
	}
	out, err := marshalJSON(page)
	if err != nil {
		s.fail("this page cannot be written as data: " + err.Error())
		return
	}
	s.handOver(fmt.Sprintf("page%03d.json", s.at), append(out, '\n'))
}

// marshalJSON is a variable for the same reason as writeCSV: json.Marshal
// refuses a few values — a NaN, an infinity, a cycle — and none of them can
// come out of a page's text, so the branch that tells somebody their page
// could not be written is otherwise unreachable.
var marshalJSON = func(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }

// extractTables is replaceable so a test can see what the workbench does when
// a page refuses to be read. The same shape as drawPage, and as `extractRuns`
// which the selection layer already declares — so this names only the one that
// was missing.
var extractTables = extract.Tables

// pageTables is the grids of the page on screen, read from the document as it
// would be saved, and whether there was anything to read.
func (s *state) pageTables() ([]extract.Table, *reader.Document, bool) {
	if s.doc == nil {
		s.fail("open a document first")
		return nil, nil, false
	}
	src, msg := s.reopen()
	if msg != "" {
		s.fail(msg)
		return nil, nil, false
	}
	tables, err := extractTables(src, s.at)
	if err != nil {
		s.fail("the grids of this page cannot be read: " + err.Error())
		return nil, nil, false
	}
	return tables, src, true
}
