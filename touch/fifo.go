// Package touch implements the touch event side of the contract.
package touch

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// CreateFIFO removes any existing file at path and makes a fresh FIFO.
func CreateFIFO(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := syscall.Mkfifo(path, 0o666); err != nil {
		return fmt.Errorf("mkfifo %s: %w", path, err)
	}
	return os.Chmod(path, 0o666)
}

// Writer writes event groups to the FIFO without ever blocking. It keeps the
// FIFO open while a reader exists.
type Writer struct {
	path    string
	fd      int
	open    bool
	Dropped int // groups lost because the pipe was full
}

func NewWriter(path string) *Writer { return &Writer{path: path, fd: -1} }

// HasReader reports whether a reader is attached right now, (re)opening if needed.
func (w *Writer) HasReader() bool {
	if w.open {
		return true
	}
	fd, err := syscall.Open(w.path, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil { // ENXIO: nobody is reading
		return false
	}
	w.fd, w.open = fd, true
	return true
}

func (w *Writer) Close() {
	if w.open {
		syscall.Close(w.fd)
		w.open = false
	}
}

// WriteGroup writes a whole group in one write (<= PIPE_BUF is atomic).
// It returns false if the reader is gone or the group was dropped.
func (w *Writer) WriteGroup(g []Event) bool {
	if !w.open {
		return false
	}
	b := make([]byte, 0, len(g)*EventSize)
	for _, e := range g {
		b = append(b, e.Bytes()...)
	}
	_, err := syscall.Write(w.fd, b)
	switch {
	case err == nil:
		return true
	case errors.Is(err, syscall.EAGAIN):
		w.Dropped++
	default: // EPIPE: reader closed
		w.Close()
	}
	return false
}
