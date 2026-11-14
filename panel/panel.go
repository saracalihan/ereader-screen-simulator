// Package panel models the e-ink panel: it only changes when an update arrives.
package panel

import (
	"fmt"
	"image"
	"sync"
	"time"

	"kobo-emu/profile"
)

type Mode int

const (
	Ideal  Mode = iota // region copied as is, no delay
	Approx             // waveform quantization, flash, timings
)

func ParseMode(s string) (Mode, error) {
	switch s {
	case "ideal", "raw", "":
		return Ideal, nil
	case "approx":
		return Approx, nil
	}
	return 0, fmt.Errorf("unknown eink mode %q (ideal|approx|raw)", s)
}

// Entry records one update for the F2 overlay and the status line.
type Entry struct {
	Rect     image.Rectangle
	Waveform string
	Full     bool
	Duration time.Duration
	At       time.Time
}

type Panel struct {
	info profile.FBInfo
	fb   []byte
	mode Mode
	conf profile.EInk

	mu          sync.RWMutex
	pix         []byte // visible RGBA, xres*yres*4, stride xres*4
	recent      []Entry
	ro, go_, bo int
}

func New(info profile.FBInfo, fb []byte, mode Mode, conf profile.EInk) *Panel {
	p := &Panel{
		info: info, fb: fb, mode: mode, conf: conf,
		pix: make([]byte, info.XRes*info.YRes*4),
		ro:  info.Red.Offset / 8, go_: info.Green.Offset / 8, bo: info.Blue.Offset / 8,
	}
	for i := range p.pix {
		p.pix[i] = 0xFF
	}
	return p
}

// Snapshot copies the panel pixels (RGBA) into dst.
func (p *Panel) Snapshot(dst []byte) {
	p.mu.RLock()
	copy(dst, p.pix)
	p.mu.RUnlock()
}

// Recent returns updates newer than maxAge.
func (p *Panel) Recent(maxAge time.Duration) []Entry {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []Entry
	for _, e := range p.recent {
		if time.Since(e.At) < maxAge {
			out = append(out, e)
		}
	}
	return out
}

// Last returns the most recent update, if any.
func (p *Panel) Last() (Entry, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.recent) == 0 {
		return Entry{}, false
	}
	return p.recent[len(p.recent)-1], true
}

// Update copies rect from the framebuffer into the panel. It returns how
// long the (emulated) update takes; the caller reports "done" after that.
func (p *Panel) Update(r image.Rectangle, wf string, full bool) (time.Duration, error) {
	r = r.Intersect(image.Rect(0, 0, p.info.XRes, p.info.YRes))
	if r.Empty() {
		return 0, fmt.Errorf("empty rect after clipping")
	}
	if p.mode == Approx {
		if _, ok := p.conf.DurationsMS[wf]; !ok {
			return 0, fmt.Errorf("unknown waveform %q", wf)
		}
	}
	content := p.read(r, wf)

	var dur time.Duration
	if p.mode == Approx {
		dur = time.Duration(p.conf.DurationsMS[wf]) * time.Millisecond
		if full {
			flash := time.Duration(p.conf.FullFlashMS) * time.Millisecond
			dur += flash
			// black, then white, then the content
			p.fill(r, 0x00)
			time.AfterFunc(flash/3, func() { p.fill(r, 0xFF) })
			time.AfterFunc(2*flash/3, func() { p.write(r, content) })
		} else {
			p.write(r, content)
		}
	} else {
		p.write(r, content)
	}

	p.mu.Lock()
	p.recent = append(p.recent, Entry{Rect: r, Waveform: wf, Full: full, Duration: dur, At: time.Now()})
	if len(p.recent) > 64 {
		p.recent = p.recent[len(p.recent)-64:]
	}
	p.mu.Unlock()
	return dur, nil
}

// read converts rect from the fb layout to RGBA, applying the waveform model.
func (p *Panel) read(r image.Rectangle, wf string) []byte {
	w := r.Dx()
	out := make([]byte, w*r.Dy()*4)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		src := p.fb[y*p.info.LineLength+r.Min.X*4:]
		dst := out[(y-r.Min.Y)*w*4:]
		for x := 0; x < w; x++ {
			R, G, B := src[x*4+p.ro], src[x*4+p.go_], src[x*4+p.bo]
			if p.mode == Approx {
				R, G, B = quantize(wf, R, G, B)
			}
			dst[x*4], dst[x*4+1], dst[x*4+2], dst[x*4+3] = R, G, B, 0xFF
		}
	}
	return out
}

func quantize(wf string, r, g, b uint8) (uint8, uint8, uint8) {
	switch wf {
	case "DU", "A2":
		if luma(r, g, b) < 128 {
			return 0, 0, 0
		}
		return 255, 255, 255
	case "GC16", "GL16", "INIT":
		v := uint8(luma(r, g, b) / 17 * 17) // 16 gray levels
		return v, v, v
	}
	return r, g, b // GLR16, GLD16 keep color
}

func luma(r, g, b uint8) int { return (299*int(r) + 587*int(g) + 114*int(b)) / 1000 }

func (p *Panel) write(r image.Rectangle, rgba []byte) {
	w := r.Dx()
	p.mu.Lock()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		copy(p.pix[(y*p.info.XRes+r.Min.X)*4:(y*p.info.XRes+r.Max.X)*4], rgba[(y-r.Min.Y)*w*4:(y-r.Min.Y+1)*w*4])
	}
	p.mu.Unlock()
}

func (p *Panel) fill(r image.Rectangle, v byte) {
	w := r.Dx()
	row := make([]byte, w*4)
	for i := range row {
		row[i] = v
	}
	for i := 3; i < len(row); i += 4 {
		row[i] = 0xFF
	}
	p.mu.Lock()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		copy(p.pix[(y*p.info.XRes+r.Min.X)*4:], row)
	}
	p.mu.Unlock()
}
