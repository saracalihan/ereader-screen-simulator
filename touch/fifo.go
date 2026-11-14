// Package touch implements the touch event side of the contract.
package touch

import (
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
