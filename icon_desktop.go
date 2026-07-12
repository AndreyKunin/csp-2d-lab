//go:build desktop

package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"runtime"
)

func generateIcon() []byte {
	switch runtime.GOOS {
	case "windows":
		return generateIconWindows()
	default:
		return generateIconDarwin()
	}
}

func generateIconWindows() []byte {
	png1 := generateCirclePNG(16)
	png2 := generateCirclePNG(32)
	return buildICO([][]byte{png1, png2})
}

func generateIconDarwin() []byte {
	return generateCirclePNG(32)
}

func generateCirclePNG(size int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size)/2 - 1
	fill := color.NRGBA{R: 70, G: 130, B: 220, A: 255}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx + 0.5
			dy := float64(y) - cy + 0.5
			if math.Sqrt(dx*dx+dy*dy) <= radius {
				img.SetNRGBA(x, y, fill)
			}
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func buildICO(pngs [][]byte) []byte {
	var buf bytes.Buffer

	binary.Write(&buf, binary.LittleEndian, uint16(0))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(len(pngs)))

	offset := uint32(6 + 16*len(pngs))
	for _, p := range pngs {
		cfg, _ := png.DecodeConfig(bytes.NewReader(p))
		w, h := cfg.Width, cfg.Height
		if w >= 256 {
			w = 0
		}
		if h >= 256 {
			h = 0
		}
		binary.Write(&buf, binary.LittleEndian, uint8(w))
		binary.Write(&buf, binary.LittleEndian, uint8(h))
		binary.Write(&buf, binary.LittleEndian, uint8(0))
		binary.Write(&buf, binary.LittleEndian, uint8(0))
		binary.Write(&buf, binary.LittleEndian, uint16(1))
		binary.Write(&buf, binary.LittleEndian, uint16(32))
		binary.Write(&buf, binary.LittleEndian, uint32(len(p)))
		binary.Write(&buf, binary.LittleEndian, offset)
		offset += uint32(len(p))
	}
	for _, p := range pngs {
		buf.Write(p)
	}
	return buf.Bytes()
}
