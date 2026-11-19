package gfx

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func ev(typ, code uint16, val int32) []byte {
	b := make([]byte, 16)
	binary.LittleEndian.PutUint16(b[8:], typ)
	binary.LittleEndian.PutUint16(b[10:], code)
	binary.LittleEndian.PutUint32(b[12:], uint32(val))
	return b
}

// group builds a Protocol A group: optional press keys, one MT block, SYN_REPORT.
func group(press bool, rx, ry int) []byte {
	var g []byte
	if press {
		g = append(g, ev(evKey, 325, 1)...)
		g = append(g, ev(evKey, btnTouch, 1)...)
	}
	g = append(g, ev(evAbs, 57, 0)...)
	g = append(g, ev(evAbs, absMTX, int32(rx))...)
	g = append(g, ev(evAbs, absMTY, int32(ry))...)
	g = append(g, ev(evSyn, synMTReport, 0)...)
	return append(g, ev(evSyn, 0, 0)...)
}

func release() []byte {
	g := ev(evAbs, absX, 0)
	g = append(g, ev(evAbs, 24, 0)...)
	g = append(g, ev(evKey, btnTouch, 0)...)
	g = append(g, ev(evKey, 325, 0)...)
	return append(g, ev(evSyn, 0, 0)...)
}

func TestTouchFIFO(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fb0.json"), []byte(`{"xres":1072,"yres":1448,"bits_per_pixel":32,"line_length":4288,"smem_len":6243328,
	"red":{"offset":0,"length":8},"green":{"offset":8,"length":8},"blue":{"offset":16,"length":8},"transp":{"offset":24,"length":8}}`), 0o644)
	os.WriteFile(filepath.Join(dir, "devices.json"), []byte(`[{"path":"`+dir+`/event1","abs":{"ABS_MT_POSITION_X":{"max":1447},"ABS_MT_POSITION_Y":{"max":1071}}}]`), 0o644)
	f, _ := os.Create(filepath.Join(dir, "fb0"))
	f.Truncate(6243328)
	f.Close()
	fifo := filepath.Join(dir, "event1")
	if err := syscall.Mkfifo(fifo, 0o666); err != nil {
		t.Fatal(err)
	}
	if err := Init(filepath.Join(dir, "fb0")); err != nil {
		t.Fatal(err)
	}
	defer Close()
	if err := InitTouch(""); err != nil { // auto: event1 next to fb0
		t.Fatal(err)
	}
	// screen (500, 700) <- raw X=700 (screen Y), raw Y=1071-500=571
	w, _ := os.OpenFile(fifo, os.O_WRONLY, 0)
	defer w.Close()
	in := Rect{450, 650, 100, 100}
	out := Rect{0, 0, 100, 100}

	w.Write(group(true, 700, 571))
	if !WaitTouch(time.Second) { // updates the state itself
		t.Fatal("no event")
	}
	if !IsTouchPressed(in) || !IsTouchDown(in) || IsTouchPressed(out) || IsTouchDown(out) {
		t.Fatalf("press: %+v", snap)
	}
	UpdateTouch() // next frame: still down, no new press
	if IsTouchPressed(in) || !IsTouchDown(in) || GetTouchPointCount() != 1 {
		t.Fatal("hold")
	}
	w.Write(release())
	WaitTouch(time.Second)
	if !IsTouchReleased(in) || IsTouchDown(in) || GetTouchPointCount() != 0 {
		t.Fatalf("release: %+v", snap)
	}
	// a quick tap between two UpdateTouch calls is not lost
	w.Write(append(group(true, 700, 571), release()...))
	time.Sleep(50 * time.Millisecond)
	UpdateTouch()
	if !IsTouchPressed(in) || !IsTouchReleased(in) {
		t.Fatal("tap lost")
	}
}
