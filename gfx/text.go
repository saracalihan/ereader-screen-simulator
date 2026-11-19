package gfx

import (
	"image"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Font selects one of the built-in typefaces (Go fonts; they cover Latin
// including Turkish characters such as ğ ü ş ı ö ç İ).
type Font int

const (
	FontRegular Font = iota
	FontBold
	FontMono
)

var (
	parsed   = map[Font]*opentype.Font{}
	faces    = map[faceKey]font.Face{}
	faceLock sync.Mutex
)

type faceKey struct {
	f    Font
	size int
}

func getFace(f Font, size int) font.Face {
	faceLock.Lock()
	defer faceLock.Unlock()
	if size < 1 {
		size = 1
	}
	k := faceKey{f, size}
	if fc, ok := faces[k]; ok {
		return fc
	}
	of, ok := parsed[f]
	if !ok {
		data := goregular.TTF
		switch f {
		case FontBold:
			data = gobold.TTF
		case FontMono:
			data = gomono.TTF
		}
		var err error
		if of, err = opentype.Parse(data); err != nil {
			panic("gfx: built-in font: " + err.Error()) // embedded data, cannot fail
		}
		parsed[f] = of
	}
	fc, err := opentype.NewFace(of, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic("gfx: face: " + err.Error())
	}
	faces[k] = fc
	return fc
}

// DrawText draws text with the regular font; (x, y) is the top-left corner
// and size the font height in pixels. '\n' starts a new line.
func DrawText(text string, x, y, size int, c Color) { DrawTextEx(FontRegular, text, x, y, size, c) }

// DrawTextEx is DrawText with an explicit font.
func DrawTextEx(f Font, text string, x, y, size int, c Color) {
	face := getFace(f, size)
	m := face.Metrics()
	lineH := m.Height.Ceil()
	for i, line := range strings.Split(text, "\n") {
		dot := fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y+i*lineH) + m.Ascent}
		var prev rune = -1
		for _, r := range line {
			if prev >= 0 {
				dot.X += face.Kern(prev, r)
			}
			dr, mask, mp, adv, ok := face.Glyph(dot, r)
			if ok {
				drawMask(dr, mask, mp, c)
			}
			dot.X += adv
			prev = r
		}
	}
}

func drawMask(dr image.Rectangle, mask image.Image, mp image.Point, c Color) {
	for yy := dr.Min.Y; yy < dr.Max.Y; yy++ {
		for xx := dr.Min.X; xx < dr.Max.X; xx++ {
			_, _, _, a := mask.At(mp.X+xx-dr.Min.X, mp.Y+yy-dr.Min.Y).RGBA()
			if a == 0 {
				continue
			}
			p := c
			p.A = uint8(uint32(c.A) * (a >> 8) / 255)
			DrawPixel(xx, yy, p)
		}
	}
}

// MeasureText returns the width in pixels of the widest line.
func MeasureText(text string, size int) int { return MeasureTextEx(FontRegular, text, size) }

func MeasureTextEx(f Font, text string, size int) int {
	face := getFace(f, size)
	best := 0
	for _, line := range strings.Split(text, "\n") {
		w := font.MeasureString(face, line).Ceil()
		best = max(best, w)
	}
	return best
}
