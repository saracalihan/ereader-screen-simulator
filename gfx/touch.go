package gfx

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Rect is a screen rectangle in pixels.
type Rect struct{ X, Y, W, H int }

// Contains reports whether the point is inside the rectangle.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H
}

const (
	maxFingers = 10
	maxEdges   = 16
)

type tpt struct{ x, y int }

// touchState is one published snapshot of the touch screen.
type touchState struct {
	down      bool
	n         int
	pts       [maxFingers]tpt
	pressed   [maxEdges]tpt // fingers that went down since the last UpdateTouch
	nPressed  int
	released  [maxEdges]tpt // fingers that lifted since the last UpdateTouch
	nReleased int
}

var (
	tmu    sync.Mutex
	live   touchState // written by the reader goroutine
	snap   touchState // what the Is* functions see; refreshed by UpdateTouch
	tfile  *os.File
	tgrab  bool
	tfd    int
	notify chan struct{} // signalled when a group was published

	// raw -> screen mapping (defaults: Kobo Clara Colour)
	swapXY, mirrorX, mirrorY = true, true, false
	rawMaxX, rawMaxY         int
)

// SetTouchTransform sets how raw touch coordinates map to the screen: the
// device's raw->screen mapping (swap X/Y first, then mirror). The default is
// the Clara Colour's: swap + mirror X. Call it after InitTouch.
func SetTouchTransform(swap, mirX, mirY bool) {
	tmu.Lock()
	swapXY, mirrorX, mirrorY = swap, mirX, mirY
	tmu.Unlock()
}

// InitTouch starts listening for touch events. Call Init first.
//
// path is the touch event device ("/dev/input/event1") or the emulator's FIFO
// ("/tmp/kobo-emu/event1"). "" picks automatically: the "cyttsp5_mt" device
// on a Kobo, or event1 next to the emulator's fb0.
//
// On a real device the touch screen is grabbed (EVIOCGRAB): while the program
// runs, Nickel and every other process receive no touch events. They return
// when CloseTouch/Close is called or the process exits.
func InitTouch(path string) error {
	if buf == nil {
		return errors.New("gfx: call Init before InitTouch")
	}
	if tfile != nil {
		return errors.New("gfx: touch already initialised")
	}
	if path == "" {
		if fbIsDev {
			p, err := discoverTouch("cyttsp5_mt")
			if err != nil {
				return err
			}
			path = p
		} else {
			path = filepath.Join(fbDir, "event1")
		}
	}
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	isDev := st.Mode()&os.ModeCharDevice != 0

	// Non-blocking so Close can wake the reader. A FIFO is opened O_RDWR: that
	// never blocks and never yields EOF when the writer goes away, and it makes
	// the emulator see a reader.
	flags := syscall.O_RDONLY
	if !isDev {
		flags = syscall.O_RDWR
	}
	fd, err := syscall.Open(path, flags|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	if isDev {
		if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), evioGrab, 1); e != 0 {
			syscall.Close(fd)
			return fmt.Errorf("EVIOCGRAB %s: %w", path, e)
		}
		tgrab = true
	}
	tfd = fd
	rawMaxX, rawMaxY = axisRanges(fd, path, isDev)
	tfile = os.NewFile(uintptr(fd), path)
	notify = make(chan struct{}, 1)
	live, snap = touchState{}, touchState{}
	go readTouch(tfile)
	return nil
}

// CloseTouch stops listening and, on a real device, releases the grab.
func CloseTouch() {
	if tfile == nil {
		return
	}
	if tgrab {
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(tfd), evioGrab, 0) // not tfile.Fd(): that would switch it to blocking mode
		tgrab = false
	}
	tfile.Close() // wakes the reader
	tfile = nil
}

// UpdateTouch publishes the touches received since the last call; "pressed"
// and "released" report what happened in between. WaitTouch calls it for you,
// so call it yourself only in a polling loop that does not use WaitTouch.
func UpdateTouch() {
	tmu.Lock()
	snap = live
	live.nPressed, live.nReleased = 0, 0
	tmu.Unlock()
}

// WaitTouch blocks until a touch event arrives or the timeout passes (0 =
// wait forever), then calls UpdateTouch itself, so the Is* functions can be
// used right after it. It reports whether an event arrived. Use it instead of
// a busy loop: the CPU sleeps until something happens.
func WaitTouch(timeout time.Duration) bool {
	if notify == nil {
		return false
	}
	got := true
	if timeout <= 0 {
		<-notify
	} else {
		t := time.NewTimer(timeout)
		select {
		case <-notify:
		case <-t.C:
			got = false
		}
		t.Stop()
	}
	UpdateTouch()
	return got
}

func anyIn(ps []tpt, r Rect) bool {
	for _, p := range ps {
		if r.Contains(p.x, p.y) {
			return true
		}
	}
	return false
}

// IsTouchDown reports whether a finger is currently on the screen inside r.
func IsTouchDown(r Rect) bool { return snap.down && anyIn(snap.pts[:snap.n], r) }

// IsTouchPressed reports whether a finger touched down inside r since the last UpdateTouch.
func IsTouchPressed(r Rect) bool { return anyIn(snap.pressed[:snap.nPressed], r) }

// IsTouchReleased reports whether a finger lifted inside r since the last UpdateTouch.
func IsTouchReleased(r Rect) bool { return anyIn(snap.released[:snap.nReleased], r) }

// GetTouchPointCount returns the number of fingers currently down.
func GetTouchPointCount() int { return snap.n }

// GetTouchPosition returns finger i's screen position (ok=false if not down).
func GetTouchPosition(i int) (x, y int, ok bool) {
	if i < 0 || i >= snap.n {
		return 0, 0, false
	}
	return snap.pts[i].x, snap.pts[i].y, true
}

// ---- reader ----

const (
	evSyn, evKey, evAbs = 0, 1, 3
	synMTReport         = 2
	btnTouch            = 330
	absX, absY          = 0, 1
	absMTX, absMTY      = 53, 54
	evioGrab            = 0x40044590 // EVIOCGRAB = _IOW('E', 0x90, int)
)

type rawGroup struct {
	pts        [maxFingers]tpt
	n          int
	curX, curY int
	sx, sy     int // plain ABS_X / ABS_Y
	sawMT      bool
	btn        bool
}

func readTouch(f *os.File) {
	var (
		b     [16 * 128]byte
		carry int
		g     rawGroup
	)
	for {
		n, err := f.Read(b[carry:])
		if n == 0 && err != nil {
			return // closed
		}
		n += carry
		off := 0
		for ; off+16 <= n; off += 16 {
			typ := binary.LittleEndian.Uint16(b[off+8:])
			code := binary.LittleEndian.Uint16(b[off+10:])
			val := int(int32(binary.LittleEndian.Uint32(b[off+12:])))
			switch typ {
			case evAbs:
				switch code {
				case absMTX:
					g.curX = val
				case absMTY:
					g.curY = val
				case absX:
					g.sx = val
				case absY:
					g.sy = val
				}
			case evKey:
				if code == btnTouch {
					g.btn = val != 0
				}
			case evSyn:
				switch code {
				case synMTReport:
					g.sawMT = true
					if g.n < maxFingers {
						g.pts[g.n] = tpt{g.curX, g.curY}
						g.n++
					}
				case 0: // SYN_REPORT
					publish(&g)
					g.n, g.sawMT = 0, false
				}
			}
		}
		carry = copy(b[:], b[off:n])
	}
}

// publish converts a finished group to screen coordinates and updates live.
func publish(g *rawGroup) {
	n := g.n
	pts := g.pts
	if n == 0 && !g.sawMT && g.btn { // device without MT blocks: single touch
		pts[0], n = tpt{g.sx, g.sy}, 1
	}

	tmu.Lock()
	prev := live.n
	if n > 0 {
		for i := 0; i < n; i++ {
			live.pts[i] = toScreen(pts[i])
		}
		for i := prev; i < n && live.nPressed < maxEdges; i++ { // new fingers
			live.pressed[live.nPressed] = live.pts[i]
			live.nPressed++
		}
		for i := n; i < prev && live.nReleased < maxEdges; i++ { // lifted fingers
			live.released[live.nReleased] = live.pts[i]
			live.nReleased++
		}
		live.n, live.down = n, true
	} else if live.down {
		for i := 0; i < prev && live.nReleased < maxEdges; i++ {
			live.released[live.nReleased] = live.pts[i]
			live.nReleased++
		}
		live.n, live.down = 0, false
	} else {
		tmu.Unlock()
		return // nothing touched (e.g. a power key report)
	}
	tmu.Unlock()
	select {
	case notify <- struct{}{}:
	default:
	}
}

// toScreen maps raw landscape coordinates to portrait screen pixels. Caller holds tmu.
func toScreen(p tpt) tpt {
	a, b := p.x, p.y
	aMax, bMax := rawMaxX, rawMaxY
	if swapXY {
		a, b = b, a
		aMax, bMax = bMax, aMax
	}
	x, y := scale(a, aMax, info.XRes-1), scale(b, bMax, info.YRes-1)
	if mirrorX {
		x = info.XRes - 1 - x
	}
	if mirrorY {
		y = info.YRes - 1 - y
	}
	return tpt{x, y}
}

func scale(v, from, to int) int {
	if from <= 0 {
		return min(max(v, 0), to)
	}
	return min(max(v*to/from, 0), to)
}

// ---- discovery and axis ranges ----

// discoverTouch finds /dev/input/eventN for a device name via /proc/bus/input/devices.
func discoverTouch(name string) (string, error) {
	b, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return "", err
	}
	for _, block := range strings.Split(string(b), "\n\n") {
		if !strings.Contains(block, `Name="`+name+`"`) {
			continue
		}
		for _, line := range strings.Split(block, "\n") {
			if !strings.HasPrefix(line, "H: Handlers=") {
				continue
			}
			for _, h := range strings.Fields(line) {
				if strings.HasPrefix(h, "event") {
					return "/dev/input/" + h, nil
				}
			}
		}
	}
	return "", fmt.Errorf("gfx: input device %q not found", name)
}

type absInfo struct{ Value, Min, Max, Fuzz, Flat, Res int32 }

// axisRanges returns the raw max of ABS_MT_POSITION_X/Y (via EVIOCGABS on a
// device, devices.json for the emulator FIFO), or screen-based defaults.
func axisRanges(fd int, path string, isDev bool) (mx, my int) {
	if isDev {
		get := func(code uintptr) int {
			var a absInfo
			req := uintptr(2<<30) | unsafe.Sizeof(a)<<16 | 'E'<<8 | (0x40 + code)
			if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(&a))); e != 0 {
				return 0
			}
			return int(a.Max)
		}
		mx, my = get(absMTX), get(absMTY)
	} else if j, err := os.ReadFile(filepath.Join(filepath.Dir(path), "devices.json")); err == nil {
		var devs []struct {
			Path string `json:"path"`
			Abs  map[string]struct {
				Max int `json:"max"`
			} `json:"abs"`
		}
		if json.Unmarshal(j, &devs) == nil {
			for _, d := range devs {
				if filepath.Base(d.Path) == filepath.Base(path) {
					mx, my = d.Abs["ABS_MT_POSITION_X"].Max, d.Abs["ABS_MT_POSITION_Y"].Max
				}
			}
		}
	}
	if mx <= 0 || my <= 0 { // fall back to the panel size (landscape)
		mx, my = max(info.XRes, info.YRes)-1, min(info.XRes, info.YRes)-1
	}
	return mx, my
}
