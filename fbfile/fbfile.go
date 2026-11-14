// Package fbfile creates the shared framebuffer file and its metadata.
package fbfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"kobo-emu/profile"
)

// FB is a memory-mapped framebuffer file.
type FB struct {
	Path string
	Buf  []byte
	f    *os.File
}

// Create (re)creates dir/fb0 with exactly smem_len bytes filled with 0xFF,
// maps it MAP_SHARED, and writes dir/fb0.json.
func Create(dir string, info profile.FBInfo) (*FB, error) {
	path := filepath.Join(dir, "fb0")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return nil, err
	}
	if err := f.Truncate(int64(info.SmemLen)); err != nil {
		f.Close()
		return nil, err
	}
	buf, err := syscall.Mmap(int(f.Fd()), 0, info.SmemLen,
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("mmap %s: %w", path, err)
	}
	for i := range buf {
		buf[i] = 0xFF
	}
	if err := writeJSON(filepath.Join(dir, "fb0.json"), info); err != nil {
		syscall.Munmap(buf)
		f.Close()
		return nil, err
	}
	return &FB{Path: path, Buf: buf, f: f}, nil
}

func (fb *FB) Close() error {
	syscall.Munmap(fb.Buf)
	return fb.f.Close()
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// WriteJSON is exported for devices.json.
func WriteJSON(path string, v any) error { return writeJSON(path, v) }
