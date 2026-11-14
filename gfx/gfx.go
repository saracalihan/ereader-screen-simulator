package gfx

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
)

// Global state, as in raylib: one framebuffer per process.
var (
	info            fbInfo
	buf             []byte
	file            *os.File
	refresh         func(x, y, w, h int) error // nil for the emulator (no refresh needed yet)
	ro, go_, bo, ao int
)

// Init opens the framebuffer file at fbPath. Two kinds of path work:
//
//	"/dev/fb0"            real device: geometry via ioctl, Refresh sends hwtcon updates
//	"/tmp/kobo-emu/fb0"   emulator file: geometry read from the fb0.json next to it
//
// Call it first; every other function draws into this framebuffer.
func Init(fbPath string) error {
	f, err := os.OpenFile(fbPath, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	if st.Mode()&os.ModeCharDevice != 0 {
		info, buf, refresh, err = openDev(f)
		if err != nil {
			f.Close()
			return err
		}
	} else {
		b, err := os.ReadFile(filepath.Join(filepath.Dir(fbPath), "fb0.json"))
		if err != nil {
			f.Close()
			return fmt.Errorf("fb info: %w", err)
		}
		if err := json.Unmarshal(b, &info); err != nil {
			f.Close()
			return err
		}
		buf, err = syscall.Mmap(int(f.Fd()), 0, info.SmemLen, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
		if err != nil {
			f.Close()
			return fmt.Errorf("mmap: %w", err)
		}
		refresh = nil
	}
	if info.BPP != 32 {
		return fmt.Errorf("only 32 bpp supported, got %d", info.BPP)
	}
	ro, go_, bo, ao = info.Red.Offset/8, info.Green.Offset/8, info.Blue.Offset/8, info.Transp.Offset/8
	file = f
	return nil
}

// Close unmaps the framebuffer.
func Close() {
	if buf != nil {
		syscall.Munmap(buf)
		buf = nil
	}
	if file != nil {
		file.Close()
		file = nil
	}
}

func GetScreenWidth() int  { return info.XRes }
func GetScreenHeight() int { return info.YRes }

// EndDrawing pushes the whole screen to the panel (real device only; the
// emulator's raw-fb window shows writes immediately).
func EndDrawing() error { return RefreshRect(0, 0, info.XRes, info.YRes) }

// RefreshRect refreshes one region of the panel.
func RefreshRect(x, y, w, h int) error {
	if refresh == nil {
		return nil
	}
	x, y, w, h = clipRect(x, y, w, h)
	if w <= 0 || h <= 0 {
		return nil
	}
	return refresh(x, y, w, h)
}

func clipRect(x, y, w, h int) (int, int, int, int) {
	if x < 0 {
		w += x
		x = 0
	}
	if y < 0 {
		h += y
		y = 0
	}
	if x+w > info.XRes {
		w = info.XRes - x
	}
	if y+h > info.YRes {
		h = info.YRes - y
	}
	return x, y, w, h
}

func put(p int, c Color) {
	if c.A == 255 {
		buf[p+ro], buf[p+go_], buf[p+bo], buf[p+ao] = c.R, c.G, c.B, 255
		return
	}
	if c.A == 0 {
		return
	}
	a, ia := int(c.A), 255-int(c.A)
	buf[p+ro] = uint8((int(c.R)*a + int(buf[p+ro])*ia) / 255)
	buf[p+go_] = uint8((int(c.G)*a + int(buf[p+go_])*ia) / 255)
	buf[p+bo] = uint8((int(c.B)*a + int(buf[p+bo])*ia) / 255)
	buf[p+ao] = 255
}

// ClearBackground fills the entire screen.
func ClearBackground(c Color) { DrawRectangle(0, 0, info.XRes, info.YRes, c) }

// DrawPixel draws one pixel; out-of-screen coordinates are ignored.
func DrawPixel(x, y int, c Color) {
	if x < 0 || y < 0 || x >= info.XRes || y >= info.YRes {
		return
	}
	put(y*info.LineLength+x*4, c)
}

// GetPixel reads a pixel back.
func GetPixel(x, y int) Color {
	if x < 0 || y < 0 || x >= info.XRes || y >= info.YRes {
		return BLANK
	}
	p := y*info.LineLength + x*4
	return Color{buf[p+ro], buf[p+go_], buf[p+bo], 255}
}

// DrawRectangle draws a filled rectangle.
func DrawRectangle(x, y, w, h int, c Color) {
	x, y, w, h = clipRect(x, y, w, h)
	for yy := y; yy < y+h; yy++ {
		p := yy*info.LineLength + x*4
		for xx := 0; xx < w; xx++ {
			put(p+xx*4, c)
		}
	}
}

// DrawRectangleLines draws a rectangle outline of the given thickness (inside the rect).
func DrawRectangleLines(x, y, w, h, thick int, c Color) {
	if thick*2 >= w || thick*2 >= h {
		DrawRectangle(x, y, w, h, c)
		return
	}
	DrawRectangle(x, y, w, thick, c)
	DrawRectangle(x, y+h-thick, w, thick, c)
	DrawRectangle(x, y+thick, thick, h-2*thick, c)
	DrawRectangle(x+w-thick, y+thick, thick, h-2*thick, c)
}

// DrawLine draws a 1-pixel line (Bresenham).
func DrawLine(x0, y0, x1, y1 int, c Color) {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	e := dx + dy
	for {
		DrawPixel(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}

// DrawLineEx draws a line with thickness (square brush; fine for e-ink UI use).
func DrawLineEx(x0, y0, x1, y1, thick int, c Color) {
	if thick <= 1 {
		DrawLine(x0, y0, x1, y1, c)
		return
	}
	r := thick / 2
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	e := dx + dy
	for {
		DrawCircle(x0, y0, r, c)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}

// DrawCircle draws a filled circle.
func DrawCircle(cx, cy, r int, c Color) {
	for dy := -r; dy <= r; dy++ {
		dx := isqrt(r*r - dy*dy)
		DrawRectangle(cx-dx, cy+dy, 2*dx+1, 1, c)
	}
}

// DrawCircleLines draws a 1-pixel circle outline (midpoint algorithm).
func DrawCircleLines(cx, cy, r int, c Color) {
	x, y, d := r, 0, 1-r
	for x >= y {
		for _, p := range [8][2]int{{x, y}, {y, x}, {-y, x}, {-x, y}, {-x, -y}, {-y, -x}, {y, -x}, {x, -y}} {
			DrawPixel(cx+p[0], cy+p[1], c)
		}
		y++
		if d < 0 {
			d += 2*y + 1
		} else {
			x--
			d += 2*(y-x) + 1
		}
	}
}

// DrawTriangle draws a filled triangle.
func DrawTriangle(x0, y0, x1, y1, x2, y2 int, c Color) {
	minY, maxY := min(y0, y1, y2), max(y0, y1, y2)
	for y := minY; y <= maxY; y++ {
		lo, hi := 1<<30, -(1 << 30)
		for _, e := range [3][4]int{{x0, y0, x1, y1}, {x1, y1, x2, y2}, {x2, y2, x0, y0}} {
			ax, ay, bx, by := e[0], e[1], e[2], e[3]
			if ay == by {
				if y == ay {
					lo, hi = min(lo, ax, bx), max(hi, ax, bx)
				}
				continue
			}
			if (y < ay && y < by) || (y > ay && y > by) {
				continue
			}
			x := ax + (bx-ax)*(y-ay)/(by-ay)
			lo, hi = min(lo, x), max(hi, x)
		}
		if hi >= lo {
			DrawRectangle(lo, y, hi-lo+1, 1, c)
		}
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func sign(a int) int {
	switch {
	case a < 0:
		return -1
	case a > 0:
		return 1
	}
	return 0
}

func isqrt(n int) int {
	if n <= 0 {
		return 0
	}
	x := int(float64Sqrt(float64(n)))
	for x*x > n {
		x--
	}
	for (x+1)*(x+1) <= n {
		x++
	}
	return x
}

func float64Sqrt(f float64) float64 { return math.Sqrt(f) }
