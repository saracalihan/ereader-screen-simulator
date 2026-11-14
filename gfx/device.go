// Package gfx is a small raylib-style 2D drawing library that renders
// directly into a Kobo (or kobo-emu) framebuffer.
package gfx

// Real framebuffer support for the Kobo (32-bit ARM layouts, hwtcon driver).

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

type bitfield struct{ Offset, Length, MsbRight uint32 }

type varInfo struct {
	Xres, Yres, XresVirtual, YresVirtual, Xoffset, Yoffset uint32
	BitsPerPixel, Grayscale                                uint32
	Red, Green, Blue, Transp                               bitfield
	Rest                                                   [20]uint32 // remainder up to 160 bytes
}

type fixInfo struct {
	ID                            [16]byte
	SmemStart, SmemLen            uint32
	Type, TypeAux, Visual         uint32
	Xpanstep, Ypanstep, Ywrapstep uint16
	_                             uint16
	LineLength                    uint32
	Rest                          [3]uint32
	Capabilities                  uint16
	Reserved                      [2]uint16
}

type updateData struct {
	Top, Left, Width, Height uint32
	Waveform, Mode, Marker   uint32
	Flags                    uint32
	Dither                   int32
}

func ioctl(fd uintptr, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

// openDev opens a real fb device and returns its info, mapping and refresh func.
func openDev(f *os.File) (fbInfo, []byte, func(x, y, w, h int, wf Waveform, full bool) error, error) {
	var zero fbInfo
	var v varInfo
	var x fixInfo
	if unsafe.Sizeof(v) != 160 || unsafe.Sizeof(x) != 68 {
		return zero, nil, nil, fmt.Errorf("struct layout mismatch (build for GOARCH=arm)")
	}
	if err := ioctl(f.Fd(), 0x4600, unsafe.Pointer(&v)); err != nil {
		return zero, nil, nil, fmt.Errorf("FBIOGET_VSCREENINFO: %w", err)
	}
	if err := ioctl(f.Fd(), 0x4602, unsafe.Pointer(&x)); err != nil {
		return zero, nil, nil, fmt.Errorf("FBIOGET_FSCREENINFO: %w", err)
	}
	ch := func(b bitfield) chn { return chn{Offset: int(b.Offset), Length: int(b.Length)} }
	info := fbInfo{
		XRes: int(v.Xres), YRes: int(v.Yres),
		BPP: int(v.BitsPerPixel), LineLength: int(x.LineLength), SmemLen: int(x.SmemLen),
		Red: ch(v.Red), Green: ch(v.Green), Blue: ch(v.Blue), Transp: ch(v.Transp),
	}
	buf, err := syscall.Mmap(int(f.Fd()), 0, info.SmemLen, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return zero, nil, nil, fmt.Errorf("mmap: %w", err)
	}
	var marker uint32
	refresh := func(x, y, w, h int, wf Waveform, full bool) error {
		marker++
		// HWTCON_SEND_UPDATE = _IOW('F', 0x2E, 36); waveform and full/partial from the caller
		var mode uint32
		if full {
			mode = 1
		}
		u := updateData{Top: uint32(y), Left: uint32(x), Width: uint32(w), Height: uint32(h), Waveform: uint32(wf), Mode: mode, Marker: marker}
		req := uintptr(1<<30 | unsafe.Sizeof(u)<<16 | 'F'<<8 | 0x2E)
		if err := ioctl(f.Fd(), req, unsafe.Pointer(&u)); err != nil {
			return fmt.Errorf("HWTCON_SEND_UPDATE: %w", err)
		}
		// HWTCON_WAIT_FOR_UPDATE_COMPLETE = _IOWR('F', 0x2F, 8)
		m := [2]uint32{marker, 0}
		return ioctl(f.Fd(), uintptr(3<<30|8<<16|'F'<<8|0x2F), unsafe.Pointer(&m))
	}
	return info, buf, refresh, nil
}
