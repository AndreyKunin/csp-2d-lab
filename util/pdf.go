package util

import (
	"bytes"
	"fmt"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/linestyle"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

type PdfPart struct {
	Label  string
	Width  float64
	Length float64
	Count  int
}

type PdfSheet struct {
	Width  float64
	Length float64
}

type PdfInput struct {
	Parts    []PdfPart
	Sheets   *PdfSheet
	SawWidth float64
}

// --- Table styling helpers -------------------------------------------------------
var (
	headerFill  = &props.Color{Red: 220, Green: 220, Blue: 220} // light gray
	borderColor = &props.Color{Red: 0, Green: 0, Blue: 0}       // black
)

func headerCell() *props.Cell {
	return &props.Cell{
		BackgroundColor: headerFill,
		BorderColor:     borderColor,
		BorderType:      border.Full,
		BorderThickness: 0.2,
		LineStyle:       linestyle.Solid,
	}
}

func bodyCell() *props.Cell {
	return &props.Cell{
		BorderColor:     borderColor,
		BorderType:      border.Full,
		BorderThickness: 0.2,
		LineStyle:       linestyle.Solid,
	}
}

func headerText(align align.Type) props.Text {
	return props.Text{Style: fontstyle.Bold, Align: align, Top: 2}
}

func bodyText(align align.Type) props.Text {
	return props.Text{Align: align, Top: 1.5}
}

func BuildPdf(input *PdfInput, images [][]byte) ([]byte, error) {
	cfg := config.NewBuilder().
		WithPageNumber().
		Build()
	m := maroto.New(cfg)

	// ---- Title ----------------------------------------------------------------------
	m.AddRow(12,
		text.NewCol(12, "Layout Report", props.Text{
			Top:   3,
			Style: fontstyle.Bold,
			Align: align.Center,
			Size:  16,
		}),
	)

	// ---- Parts table ----------------------------------------------------------------
	m.AddRow(8,
		text.NewCol(12, "Parts", props.Text{
			Top:   2,
			Style: fontstyle.Bold,
			Size:  12,
		}),
	)
	// Header row: 8 of 12 units used (left-aligned, not stretched to page width).
	m.AddRow(7,
		text.NewCol(2, "Label", headerText(align.Center)).WithStyle(headerCell()),
		text.NewCol(2, "Width", headerText(align.Center)).WithStyle(headerCell()),
		text.NewCol(2, "Length", headerText(align.Center)).WithStyle(headerCell()),
		text.NewCol(2, "Count", headerText(align.Center)).WithStyle(headerCell()),
		col.New(4),
	)
	for _, p := range input.Parts {
		m.AddRow(6,
			text.NewCol(2, p.Label, bodyText(align.Center)).WithStyle(bodyCell()),
			text.NewCol(2, fmt.Sprintf("%g", p.Width), bodyText(align.Center)).WithStyle(bodyCell()),
			text.NewCol(2, fmt.Sprintf("%g", p.Length), bodyText(align.Center)).WithStyle(bodyCell()),
			text.NewCol(2, fmt.Sprintf("%d", p.Count), bodyText(align.Center)).WithStyle(bodyCell()),
			col.New(4),
		)
	}

	// ---- Sheets table (optional) ----------------------------------------------------
	if input.Sheets != nil {
		m.AddRow(8,
			text.NewCol(12, "Sheets", props.Text{
				Top:   2,
				Style: fontstyle.Bold,
				Size:  12,
			}),
		)
		// Header row: 6 of 12 units used.
		m.AddRow(7,
			text.NewCol(3, "Width", headerText(align.Center)).WithStyle(headerCell()),
			text.NewCol(3, "Length", headerText(align.Center)).WithStyle(headerCell()),
			col.New(6),
		)
		m.AddRow(6,
			text.NewCol(3, fmt.Sprintf("%g", input.Sheets.Width), bodyText(align.Center)).WithStyle(bodyCell()),
			text.NewCol(3, fmt.Sprintf("%g", input.Sheets.Length), bodyText(align.Center)).WithStyle(bodyCell()),
			col.New(6),
		)
	}

	// ---- Constraints (saw width) ---------------------------------------------
	m.AddRow(8,
		text.NewCol(12, "Constraints", props.Text{
			Top:   2,
			Style: fontstyle.Bold,
			Size:  12,
		}),
	)
	m.AddRow(7,
		text.NewCol(3, "Saw width", headerText(align.Center)).WithStyle(headerCell()),
		col.New(9),
	)
	m.AddRow(6,
		text.NewCol(3, fmt.Sprintf("%g", input.SawWidth), bodyText(align.Center)).WithStyle(bodyCell()),
		col.New(9),
	)

	// ---- Images: two per page -------------------------------------------------------
	// Each image goes into its own full-width row; the row height is chosen so
	// that two of them fit on a single A4 page (A4 useful height ~ 257 mm minus
	// margins and previous content). Maroto auto-paginates when content exceeds
	// the page, so we add a heading once and then dump rows in pairs.
	if len(images) > 0 {
		// Image row height: ~120 mm gives two per A4 page comfortably with a
		// small caption above each. Maroto will resize the image to fit inside
		// the column while preserving the aspect ratio (Center prop).
		const imageRowHeight = 120.0

		for _, imgBytes := range images {
			m.AddRow(imageRowHeight,
				col.New(12).Add(
					image.NewFromBytes(imgBytes, extension.Png, props.Rect{
						Center:  true,
						Percent: 95,
					}),
				),
			)
		}
	}

	// ---- Footer ---------------------------------------------------------------------
	_ = row.New(0) // (keep row import alive if other parts are removed)

	doc, err := m.Generate()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := buf.Write(doc.GetBytes()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
