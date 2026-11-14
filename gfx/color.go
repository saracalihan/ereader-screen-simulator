package gfx

// Color is an RGBA color. A < 255 is alpha-blended over the existing pixel.
type Color struct{ R, G, B, A uint8 }

// Basic colors (names follow raylib).
var (
	BLANK     = Color{0, 0, 0, 0}
	BLACK     = Color{0, 0, 0, 255}
	WHITE     = Color{255, 255, 255, 255}
	LIGHTGRAY = Color{200, 200, 200, 255}
	GRAY      = Color{130, 130, 130, 255}
	DARKGRAY  = Color{80, 80, 80, 255}
	RED       = Color{230, 41, 55, 255}
	MAROON    = Color{190, 33, 55, 255}
	ORANGE    = Color{255, 161, 0, 255}
	GOLD      = Color{255, 203, 0, 255}
	YELLOW    = Color{253, 249, 0, 255}
	LIME      = Color{0, 158, 47, 255}
	GREEN     = Color{0, 228, 48, 255}
	DARKGREEN = Color{0, 117, 44, 255}
	SKYBLUE   = Color{102, 191, 255, 255}
	BLUE      = Color{0, 121, 241, 255}
	DARKBLUE  = Color{0, 82, 172, 255}
	PURPLE    = Color{200, 122, 255, 255}
	VIOLET    = Color{135, 60, 190, 255}
	PINK      = Color{255, 109, 194, 255}
	MAGENTA   = Color{255, 0, 255, 255}
	BROWN     = Color{127, 106, 79, 255}
	BEIGE     = Color{211, 176, 131, 255}
	CYAN      = Color{0, 255, 255, 255}
)

// RGB returns an opaque color.
func RGB(r, g, b uint8) Color { return Color{r, g, b, 255} }

// RGBA returns a color with alpha.
func RGBA(r, g, b, a uint8) Color { return Color{r, g, b, a} }

// Fade returns c with its alpha scaled by f (0..1).
func Fade(c Color, f float64) Color {
	if f < 0 {
		f = 0
	} else if f > 1 {
		f = 1
	}
	c.A = uint8(float64(c.A) * f)
	return c
}
