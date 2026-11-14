package touch

import "kobo-emu/profile"

// Point is an integer coordinate pair.
type Point struct{ X, Y int }

// Transform maps screen pixels to raw touch coordinates: the inverse of the
// device's raw->screen mapping (swap, then mirror).
type Transform struct {
	swap, mirrorX, mirrorY bool
	rawMaxX, rawMaxY       int // raw axis maxima (landscape)
	scrW, scrH             int
}

func NewTransform(t profile.Touch, scrW, scrH, rawMaxX, rawMaxY int) Transform {
	return Transform{t.SwapXY, t.MirrorX, t.MirrorY, rawMaxX, rawMaxY, scrW, scrH}
}

// ToRaw converts a screen pixel to raw coordinates, clamped to the axis range.
func (t Transform) ToRaw(p Point) Point {
	sx, sy := clamp(p.X, 0, t.scrW-1), clamp(p.Y, 0, t.scrH-1)
	// device: sx,sy = (swap? ry,rx : rx,ry), then mirrors. Undo in reverse order.
	if t.mirrorX {
		sx = t.scrW - 1 - sx
	}
	if t.mirrorY {
		sy = t.scrH - 1 - sy
	}
	rx, ry := sx, sy
	if t.swap {
		rx, ry = sy, sx
	}
	return Point{clamp(rx, 0, t.rawMaxX), clamp(ry, 0, t.rawMaxY)}
}

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }
