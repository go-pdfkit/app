package main

import (
	"os"

	"github.com/go-pdfkit/reader"
)

// samplePDF is the document handed to the tab when none was named: three
// pages, each carrying a black square, written by the same code the workbench
// reads with. Nothing is committed to the repository that is not made here.
func samplePDF() ([]byte, error) {
	w := reader.NewWriter("1.7")
	pages := w.Reserve()
	kids := reader.Array{}
	for i := 1; i <= 3; i++ {
		content := w.Add(&reader.Stream{Dict: reader.Dict{},
			Raw: []byte("0 g 40 40 220 300 re f 1 0 0 RG 8 w 60 60 m 200 320 l S")})
		kids = append(kids, w.Add(reader.Dict{
			"Type": reader.Name("Page"), "Parent": pages, "Contents": content,
		}))
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
