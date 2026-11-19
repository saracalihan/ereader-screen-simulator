// fbsquare: demo of the gfx library.
//
//	Kobo:     fbsquare -fb /dev/fb0
//	Emulator: fbsquare            (uses /tmp/kobo-emu)
//
// Touch the colored square to change its color; touch the top-left black
// square to quit. While running, the program owns the touch screen (Nickel
// receives nothing).
package main

import (
	"flag"
	"log"

	gfx "kobogfx"
)

func main() {
	fb := flag.String("fb", "/tmp/kobo-emu/fb0", "framebuffer file (/dev/fb0 on the Kobo)")
	touch := flag.String("touch", "", `touch device or FIFO ("" = auto, "none" = disabled)`)
	flag.Parse()

	if err := gfx.Init(*fb); err != nil {
		log.Fatal(err)
	}
	defer gfx.Close()

	w, h := gfx.GetScreenWidth(), gfx.GetScreenHeight()
	gfx.ClearBackground(gfx.WHITE)

	n := 150
	gfx.DrawRectangle(0, 0, n, n, gfx.BLACK)
	gfx.DrawRectangle(w-n, 0, n, n, gfx.RED)
	gfx.DrawRectangle(0, h-n, n, n, gfx.GREEN)
	gfx.DrawRectangle(w-n, h-n, n, n, gfx.BLUE)
	gfx.DrawRectangle(w/2-n/2, h/2-n/2, n, n, gfx.GRAY)

	gfx.DrawRectangleLines(20, 20, w-40, h-40, 4, gfx.DARKGRAY)
	gfx.DrawLine(0, 0, w-1, h-1, gfx.BLACK)
	gfx.DrawLineEx(w-1, 0, 0, h-1, 6, gfx.ORANGE)
	gfx.DrawCircle(w/2, h/4, 100, gfx.PURPLE)
	gfx.DrawCircleLines(w/2, h/4, 130, gfx.BLACK)
	gfx.DrawTriangle(w/2, h*3/4-100, w/2-100, h*3/4+80, w/2+100, h*3/4+80, gfx.YELLOW)
	gfx.DrawCircle(w/2, h/2, 60, gfx.Fade(gfx.BLUE, 0.5))
	gfx.DrawPixel(w/2, h/2, gfx.BLACK)

	gfx.DrawText("Merhaba Kobo! ğüşıöçİ", 60, 200, 48, gfx.BLACK)
	gfx.DrawTextEx(gfx.FontMono, "gfx.DrawText\nçok satır", 60, 270, 32, gfx.DARKGRAY)
	gfx.DrawText("sağa dayalı", w-60-gfx.MeasureText("sağa dayalı", 36), 340, 36, gfx.BLUE)

	// touch test: a square that changes color when touched
	btn := gfx.Rect{X: 60, Y: 450, W: 220, H: 220}
	quit := gfx.Rect{X: 0, Y: 0, W: n, H: n}
	colors := []gfx.Color{gfx.RED, gfx.GREEN, gfx.BLUE, gfx.ORANGE, gfx.PURPLE}
	ci := 0
	drawBtn := func() {
		gfx.DrawRectangle(btn.X, btn.Y, btn.W, btn.H, colors[ci])
		gfx.DrawRectangleLines(btn.X, btn.Y, btn.W, btn.H, 4, gfx.BLACK)
		gfx.DrawText("dokun", btn.X+50, btn.Y+90, 40, gfx.WHITE)
	}
	drawBtn()

	if err := gfx.EndDrawing(); err != nil {
		log.Fatal(err)
	}
	if *touch == "none" {
		return
	}
	if err := gfx.InitTouch(*touch); err != nil {
		log.Fatal(err)
	}
	// (gfx.Close, deferred above, also releases the touch grab)

	for {
		gfx.WaitTouch(0) // sleeps until a touch event arrives, then updates the state
		if gfx.IsTouchPressed(quit) {
			return
		}
		if gfx.IsTouchPressed(btn) {
			ci = (ci + 1) % len(colors)
			drawBtn()
			gfx.RefreshRectEx(btn.X, btn.Y, btn.W, btn.H, gfx.GC16, false)
		}
	}
}
