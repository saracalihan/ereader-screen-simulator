// Package ui shows the framebuffer in an Ebitengine window ("raw fb" mode).
package ui

import (
	"fmt"

	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"kobo-emu/panel"
	"kobo-emu/profile"
	"kobo-emu/touch"
)

type Game struct {
	info  profile.FBInfo
	fb    []byte
	pix   []byte // RGBA, xres*yres*4
	img   *ebiten.Image
	scale float64 // 0 = auto
	panel *panel.Panel
	raw   bool // F3: show fb directly
	rects bool // F2: show recent update rectangles
	touch *touch.Controller
	dots  bool // F1: touch overlay
	trail []touch.Point
	sized bool
	// byte index of each channel inside a framebuffer pixel
	ro, go_, bo int
}

// Run blocks until the window is closed. scale 0 fits the monitor height.
func Run(info profile.FBInfo, fb []byte, p *panel.Panel, tc *touch.Controller, scale float64, raw bool) error {
	g := &Game{
		info: info, fb: fb, scale: scale, panel: p, raw: raw, touch: tc,
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
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		g.raw = !g.raw
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.dots = !g.dots
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF2) {
		g.rects = !g.rects
	}
	g.updateTouch()
	if g.raw {
		g.convert()
	} else {
		g.panel.Snapshot(g.pix)
	}
	g.img.WritePixels(g.pix)
	x, y := ebiten.CursorPosition() // already in fb pixels (Layout maps the scale)
	mode := "panel"
	if g.raw {
		mode = "raw fb"
	}
	reader, dropped := g.touch.Status()
	rd := "okuyucu yok"
	if reader {
		rd = "okuyucu var"
	}
	ebiten.SetWindowTitle(fmt.Sprintf("kobo-emu [%s]  fb=(%d,%d)  %s  dropped=%d", mode, x, y, rd, dropped))
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

// updateTouch turns mouse state into fingers. Left button = finger 0; with
// Ctrl held, finger 1 is the mirror of the cursor around the screen center.
func (g *Game) updateTouch() {
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.touch.SetFingers(nil)
		g.trail = g.trail[:0]
		return
	}
	x, y := ebiten.CursorPosition()
	fs := []touch.Finger{{ID: 0, P: touch.Point{X: x, Y: y}}}
	if ebiten.IsKeyPressed(ebiten.KeyControl) {
		fs = append(fs, touch.Finger{ID: 1, P: touch.Point{X: g.info.XRes - 1 - x, Y: g.info.YRes - 1 - y}})
	}
	g.touch.SetFingers(fs)
	g.trail = append(g.trail, fs[0].P)
	if len(g.trail) > 200 {
		g.trail = g.trail[1:]
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.DrawImage(g.img, nil)
	if g.dots {
		for i, p := range g.trail {
			a := uint8(40 + 160*i/len(g.trail))
			vector.DrawFilledCircle(screen, float32(p.X), float32(p.Y), 14, color.RGBA{G: a, B: a / 2, A: a}, true)
		}
	}
	if !g.rects {
		return
	}
	for _, e := range g.panel.Recent(2 * time.Second) {
		a := uint8(255 - 255*time.Since(e.At)/(2*time.Second))
		c := color.RGBA{R: a, A: a}
		if e.Full {
			c = color.RGBA{B: a, A: a}
		}
		vector.StrokeRect(screen, float32(e.Rect.Min.X), float32(e.Rect.Min.Y),
			float32(e.Rect.Dx()), float32(e.Rect.Dy()), 3, c, false)
	}
}

func (g *Game) Layout(_, _ int) (int, int) { return g.info.XRes, g.info.YRes }
