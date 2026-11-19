// touchdraw: draws a line wherever you touch. Works on the Kobo and in kobo-emu.
//
//	Kobo:     touchdraw -fb /dev/fb0
//	Emulator: touchdraw            (uses /tmp/kobo-emu)
//
// Uses gfx.InitTouch, so on the Kobo the touch screen is grabbed: Nickel
// receives no touches while this runs. Touch the top-left 100x100 corner to
// clear the screen, the top-right 100x100 corner to quit.
package main

import (
	"flag"
	"log"

	gfx "kobogfx"
)

func main() {
	fb := flag.String("fb", "/tmp/kobo-emu/fb0", "framebuffer file (/dev/fb0 on the Kobo)")
	dev := flag.String("touch", "", `touch device or FIFO ("" = auto)`)
	thick := flag.Int("thick", 6, "line thickness")
	flag.Parse()

	if err := gfx.Init(*fb); err != nil {
		log.Fatal(err)
	}
	defer gfx.Close() // also releases the touch grab
	if err := gfx.InitTouch(*dev); err != nil {
		log.Fatal(err)
	}

	w := gfx.GetScreenWidth()
	clearBtn := gfx.Rect{X: 0, Y: 0, W: 100, H: 100}
	quitBtn := gfx.Rect{X: w - 100, Y: 0, W: 100, H: 100}

	clear := func() {
		gfx.ClearBackground(gfx.WHITE)
		gfx.DrawRectangleLines(clearBtn.X, clearBtn.Y, clearBtn.W, clearBtn.H, 3, gfx.GRAY)
		gfx.DrawRectangleLines(quitBtn.X, quitBtn.Y, quitBtn.W, quitBtn.H, 3, gfx.RED)
		gfx.DrawText("temizle", 8, 36, 22, gfx.GRAY)
		gfx.DrawText("çık", quitBtn.X+30, 36, 22, gfx.RED)
		gfx.SetRefreshMode(gfx.GC16, true)
		gfx.EndDrawing()
		gfx.SetRefreshMode(gfx.DU, false) // fast partial updates while drawing
	}
	clear()

	var px, py int
	havePrev := false
	for {
		gfx.WaitTouch(0)
		if gfx.IsTouchPressed(quitBtn) {
			return
		}
		if gfx.IsTouchPressed(clearBtn) {
			clear()
			havePrev = false
			continue
		}
		x, y, ok := gfx.GetTouchPosition(0)
		if !ok {
			havePrev = false
			continue
		}
		if havePrev {
			gfx.DrawLineEx(px, py, x, y, *thick, gfx.BLACK)
			m := *thick + 2
			gfx.RefreshRect(min(px, x)-m, min(py, y)-m, abs(x-px)+2*m, abs(y-py)+2*m)
		}
		px, py, havePrev = x, y, true
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
