// fbsquare: demo of the gfx library. Usage: fbsquare [-fb path]
package main

import (
	"flag"
	"log"

	gfx "kobogfx"
)

func main() {
	fb := flag.String("fb", "/tmp/kobo-emu/fb0", "framebuffer file (/dev/fb0 on the Kobo)")
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

	if err := gfx.EndDrawing(); err != nil {
		log.Fatal(err)
	}
}
