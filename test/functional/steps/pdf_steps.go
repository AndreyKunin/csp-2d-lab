package steps

import (
	"bytes"
	"csp2d/test/support/httpclient"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"

	"github.com/cucumber/godog"
)

func registerPdfSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^I request a PDF for fixture "([^"]*)" with (\d+) images?$`, w.requestPdfForFixture)
	ctx.Step(`^I request a PDF with request field '([^']*)'$`, w.requestPdfWithRaw)
	ctx.Step(`^I request a PDF without the request field$`, w.requestPdfWithoutRequest)
	ctx.Step(`^I send a non-multipart PDF request$`, w.requestPdfNotMultipart)
	ctx.Step(`^the response is a PDF document$`, w.responseIsPdf)
}

// testPNG renders a small solid image.
func testPNG(i int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	c := color.RGBA{R: uint8(40 * i), G: 120, B: 200, A: 255}
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (w *World) requestPdfForFixture(id string, images int) error {
	if err := w.startRequestFromFixture(id); err != nil {
		return err
	}
	body, err := w.requestBody()
	if err != nil {
		return err
	}
	var imgs []httpclient.PdfImage
	for i := 0; i < images; i++ {
		data, err := testPNG(i)
		if err != nil {
			return err
		}
		imgs = append(imgs, httpclient.PdfImage{Name: fmt.Sprintf("sheet-%d.png", i+1), Data: data})
	}
	w.record(w.client.GeneratePdf(body, imgs))
	return nil
}

func (w *World) requestPdfWithRaw(raw string) error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	w.record(w.client.GeneratePdf([]byte(raw), nil))
	return nil
}

func (w *World) requestPdfWithoutRequest() error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	w.record(w.client.GeneratePdf(nil, nil))
	return nil
}

func (w *World) requestPdfNotMultipart() error {
	if err := w.ensureServer(); err != nil {
		return err
	}
	w.record(w.client.Do(http.MethodPost, "/api/v1/pdf/generate", []byte(`{"parts":[]}`), "application/json"))
	return nil
}

func (w *World) responseIsPdf() error {
	resp, err := w.requireStatus(http.StatusOK)
	if err != nil {
		return err
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		return fmt.Errorf("Content-Type = %q, want application/pdf", ct)
	}
	if !bytes.HasPrefix(resp.Body, []byte("%PDF-")) {
		return fmt.Errorf("body does not start with %%PDF-")
	}
	return nil
}
