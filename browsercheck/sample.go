package main

import (
	"os"

	"github.com/go-pdfkit/reader"
)

// samplePDF is the document handed to the tab when none was named: three
// pages, each carrying a black square, written by the same code the workbench
// reads with. Nothing is committed to the repository that is not made here.
//
// ⛔⛔ IT CARRIES LINKS AND WORDS NOW, because without them this check could
// not see the feature it is supposed to check. The contents pane lists a
// document's links; the sample had none, so every run of this program exercised
// the fallback-to-a-page-list path and the links path was reached by unit tests
// alone. That is the same shape as the defect this repository has already
// shipped twice: a feature whose condition the check never meets is a feature
// the check cannot see fail.
//
// The words exist so the rows have something to SAY. A link's row is named by
// the text under its rectangle, so a link over a bare square would be checked
// only for existing, not for being readable.
func samplePDF() ([]byte, error) {
	w := reader.NewWriter("1.7")
	pages := w.Reserve()
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

	refs := make([]reader.Ref, 3)
	for i := range refs {
		refs[i] = w.Reserve()
	}
	for i, ref := range refs {
		// Helvetica at 10 points with every width at 500/1000 em puts each
		// character 5 points wide, so "the second chapter" (18 characters from
		// x = 40) runs to x = 130.
		content := w.Add(&reader.Stream{Dict: reader.Dict{}, Raw: []byte(
			"0 g 40 40 220 260 re f 1 0 0 RG 8 w 60 60 m 200 280 l S\n" +
				"BT /F1 10 Tf 40 360 Td (the second chapter) Tj ET\n" +
				"BT /F1 10 Tf 40 340 Td (somewhere outside) Tj ET")})
		page := reader.Dict{
			"Type": reader.Name("Page"), "Parent": pages, "Contents": content,
			"Resources": reader.Dict{"Font": reader.Dict{"F1": font}},
			"Annots": reader.Array{
				// Over "the second chapter", to the page after this one.
				w.Add(reader.Dict{"Type": reader.Name("Annot"), "Subtype": reader.Name("Link"),
					"Rect": reader.Array{reader.Integer(40), reader.Integer(358),
						reader.Integer(130), reader.Integer(372)},
					"Dest": reader.Array{refs[(i+1)%len(refs)], reader.Name("Fit")}}),
				// Over "somewhere outside", which leaves the document.
				w.Add(reader.Dict{"Type": reader.Name("Annot"), "Subtype": reader.Name("Link"),
					"Rect": reader.Array{reader.Integer(40), reader.Integer(338),
						reader.Integer(125), reader.Integer(352)},
					"A": reader.Dict{"S": reader.Name("URI"),
						"URI": reader.String("https://example.invalid/outside")}}),
			},
		}
		w.Put(ref, page)
	}
	kids := make(reader.Array, len(refs))
	for i, r := range refs {
		kids[i] = r
	}
	w.Put(pages, reader.Dict{"Type": reader.Name("Pages"), "Kids": kids,
		"Count":    reader.Integer(len(kids)),
		"MediaBox": reader.Array{reader.Integer(0), reader.Integer(0), reader.Integer(300), reader.Integer(400)}})
	root := w.Add(reader.Dict{"Type": reader.Name("Catalog"), "Pages": pages})
	return w.Finish(reader.Dict{"Root": root})
}

// writeSample puts the built-in document somewhere the browser can open it,
// since a file input is handed a path rather than bytes.
func writeSample(dir string) (string, error) {
	data, err := samplePDF()
	if err != nil {
		return "", err
	}
	path := dir + "/sample.pdf"
	return path, os.WriteFile(path, data, 0o644)
}
