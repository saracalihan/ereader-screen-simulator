// touchdraw: draws a line wherever you touch. Works on the Kobo and in kobo-emu.
//
//	Kobo:     touchdraw -fb /dev/fb0 -touch /dev/input/event1
//	Emulator: touchdraw -fb /tmp/kobo-emu/fb0 -touch /tmp/kobo-emu/event1
//
// Clearing the screen: touch the top-left 100x100 corner.
package main

import (
	"encoding/binary"
	"flag"
	"io"
	"log"
	"os"

	gfx "kobogfx"
)

// Linux input constants.
const (
	evSyn, evKey, evAbs = 0, 1, 3
	synReport           = 0
	synMTReport         = 2
	btnTouch            = 330
	absMTX, absMTY      = 53, 54
	absX, absY          = 0, 1
)

func main() {
	fb := flag.String("fb", "/tmp/kobo-emu/fb0", "framebuffer file (/dev/fb0 on the Kobo)")
	dev := flag.String("touch", "/tmp/kobo-emu/event1", "touch event device or FIFO")
	thick := flag.Int("thick", 6, "line thickness")
	maxY := flag.Int("rawmax-y", 1071, "raw Y axis max (screen short side)")
	flag.Parse()

	if err := gfx.Init(*fb); err != nil {
		log.Fatal(err)
	}
	defer gfx.Close()

	f, err := os.Open(*dev)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	clear := func() {
		gfx.ClearBackground(gfx.WHITE)
		gfx.DrawRectangleLines(0, 0, 100, 100, 3, gfx.GRAY)
		gfx.SetRefreshMode(gfx.GC16, true)
		gfx.EndDrawing()
		gfx.SetRefreshMode(gfx.DU, false) // fast partial updates while drawing
	}
	clear()

	// Raw landscape (X: 0..maxX, Y: 0..maxY) -> screen portrait.
	// Clara Colour: screenX = maxY - rawY, screenY = rawX.
	toScreen := func(rx, ry int) (int, int) { return *maxY - ry, rx }

	var rx, ry int
	var haveX, haveY, down bool
	var px, py int
	var havePrev bool

	rec := make([]byte, 16) // Kobo input_event is always 16 bytes
	for {
		if _, err := io.ReadFull(f, rec); err != nil {
			if err == io.EOF {
				return
			}
			log.Fatal(err)
		}
		typ := binary.LittleEndian.Uint16(rec[8:])
		code := binary.LittleEndian.Uint16(rec[10:])
		val := int(int32(binary.LittleEndian.Uint32(rec[12:])))

		switch {
		case typ == evAbs && (code == absMTX || code == absX):
			rx, haveX = val, true
		case typ == evAbs && (code == absMTY || code == absY):
			ry, haveY = val, true
		case typ == evKey && code == btnTouch:
			down = val != 0
			if !down {
				havePrev = false
			}
		case typ == evSyn && code == synReport:
			if !down || !haveX || !haveY {
				continue
			}
			x, y := toScreen(rx, ry)
			if x < 100 && y < 100 {
				clear()
				havePrev = false
				continue
			}
			if havePrev {
				gfx.DrawLineEx(px, py, x, y, *thick, gfx.BLACK)
				m := *thick + 2
				x0, y0 := min(px, x)-m, min(py, y)-m
				gfx.RefreshRect(x0, y0, abs(x-px)+2*m, abs(y-py)+2*m)
			}
			px, py, havePrev = x, y, true
		}
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
