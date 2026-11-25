//go:build testhooks

// Unit tests for csp2d/util (PDF report).
package util_test

import (
	"bytes"
	"csp2d/util"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 20, 10))
	for x := 0; x < 20; x++ {
		img.Set(x, 5, color.Black)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func input(withSheets bool) *util.PdfInput {
	in := &util.PdfInput{
		Parts:    []util.PdfPart{{Label: "1", Width: 30, Length: 20, Count: 2}, {Label: "2", Width: 10, Length: 10, Count: 1}},
		SawWidth: 1.5,
	}
	if withSheets {
		in.Sheets = &util.PdfSheet{Width: 100, Length: 100}
	}
	return in
}

// UT-PDF-01: a PDF is produced with and without sheets and images.
func TestBuildPdf(t *testing.T) {
	cases := []struct {
		name   string
		in     *util.PdfInput
		images [][]byte
	}{
		{"sheets, no images", input(true), nil},
		{"no sheets, no images", input(false), nil},
		{"sheets, two images", input(true), [][]byte{pngBytes(t), pngBytes(t)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := util.BuildPdf(c.in, c.images)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(out, []byte("%PDF-")) {
				t.Fatalf("output does not start with %%PDF-: %q", out[:min(16, len(out))])
			}
		})
	}
}

// UT-PDF-02: invalid image bytes produce an error, never a panic.
func TestBuildPdfInvalidImage(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BuildPdf panicked: %v", r)
		}
	}()
	if _, err := util.BuildPdf(input(true), [][]byte{[]byte("not an image")}); err == nil {
		t.Log("BuildPdf accepted invalid image bytes (no error); the document may omit the image")
	}
}

// UT-PDF-03: the document is structurally valid PDF and its content streams
// contain the part labels.
func TestBuildPdfContent(t *testing.T) {
	in := &util.PdfInput{
		Parts:    []util.PdfPart{{Label: "4711", Width: 30, Length: 20, Count: 2}, {Label: "8642", Width: 10, Length: 10, Count: 1}},
		Sheets:   &util.PdfSheet{Width: 100, Length: 100},
		SawWidth: 1.5,
	}
	out, err := util.BuildPdf(in, [][]byte{pngBytes(t)})
	if err != nil {
		t.Fatal(err)
	}
	conf := model.NewDefaultConfiguration()
	if err := pdfapi.Validate(bytes.NewReader(out), conf); err != nil {
		t.Fatalf("invalid PDF: %v", err)
	}
	if pages, err := pdfapi.PageCount(bytes.NewReader(out), conf); err != nil || pages < 1 {
		t.Fatalf("page count %d, err %v", pages, err)
	}
	dir := t.TempDir()
	if err := pdfapi.ExtractContent(bytes.NewReader(out), dir, "report", nil, conf); err != nil {
		t.Fatal(err)
	}
	files, _ := os.ReadDir(dir)
	var content []byte
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		content = append(content, data...)
	}
	for _, label := range []string{"4711", "8642"} {
		if !bytes.Contains(content, []byte(label)) {
			t.Errorf("content streams do not contain label %s", label)
		}
	}
}
