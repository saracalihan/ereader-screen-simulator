// Package ui shows the framebuffer in an Ebitengine window ("raw fb" mode).
package ui

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

	"kobo-emu/profile"
)

type Game struct {
	info  profile.FBInfo
	fb    []byte
	pix   []byte // RGBA, xres*yres*4
	img   *ebiten.Image
	scale float64 // 0 = auto
	sized bool
	// byte index of each channel inside a framebuffer pixel
	ro, go_, bo int
}

// Run blocks until the window is closed. scale 0 fits the monitor height.
func Run(info profile.FBInfo, fb []byte, scale float64) error {
	g := &Game{
		info: info, fb: fb, scale: scale,
		pix: make([]byte, info.XRes*info.YRes*4),
		img: ebiten.NewImage(info.XRes, info.YRes),
		ro:  info.Red.Offset / 8, go_: info.Green.Offset / 8, bo: info.Blue.Offset / 8,
	}
	s := scale
	if s <= 0 {
		s = 0.5
	}
	ebiten.SetWindowSize(int(float64(info.XRes)*s), int(float64(info.YRes)*s))
	ebiten.SetWindowTitle("kobo-emu")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetTPS(30) // fb is polled ~30 times per second
	return ebiten.RunGame(g)
}

func (g *Game) Update() error {
	if !g.sized {
		g.sized = true
		if g.scale <= 0 {
			_, mh := ebiten.Monitor().Size()
			if mh > 0 {
				s := float64(mh) * 0.85 / float64(g.info.YRes)
				ebiten.SetWindowSize(int(float64(g.info.XRes)*s), int(float64(g.info.YRes)*s))
			}
		}
	}
	g.convert()
	g.img.WritePixels(g.pix)
	x, y := ebiten.CursorPosition() // already in fb pixels (Layout maps the scale)
	ebiten.SetWindowTitle(fmt.Sprintf("kobo-emu  fb=(%d,%d)", x, y))
	return nil
}

// convert copies the visible area row by row into RGBA, forcing alpha to 255.
func (g *Game) convert() {
	w, h, ll := g.info.XRes, g.info.YRes, g.info.LineLength
	for y := 0; y < h; y++ {
		src := g.fb[y*ll : y*ll+w*4]
		dst := g.pix[y*w*4 : (y+1)*w*4]
		for x := 0; x < w*4; x += 4 {
			dst[x] = src[x+g.ro]
			dst[x+1] = src[x+g.go_]
			dst[x+2] = src[x+g.bo]
			dst[x+3] = 0xFF
		}
	}
}

func (g *Game) Draw(screen *ebiten.Image) { screen.DrawImage(g.img, nil) }

func (g *Game) Layout(_, _ int) (int, int) { return g.info.XRes, g.info.YRes }
