// Package profile loads and validates a device profile.
package profile

import (
	"encoding/json"
	"fmt"
	"os"
)

type Channel struct {
	Offset int `json:"offset"`
	Length int `json:"length"`
}

// FBInfo is written verbatim to fb0.json.
type FBInfo struct {
	ID          string  `json:"id"`
	XRes        int     `json:"xres"`
	YRes        int     `json:"yres"`
	XResVirtual int     `json:"xres_virtual"`
	YResVirtual int     `json:"yres_virtual"`
	BPP         int     `json:"bits_per_pixel"`
	Grayscale   int     `json:"grayscale"`
	LineLength  int     `json:"line_length"`
	SmemLen     int     `json:"smem_len"`
	Rotate      int     `json:"rotate"`
	Red         Channel `json:"red"`
	Green       Channel `json:"green"`
	Blue        Channel `json:"blue"`
	Transp      Channel `json:"transp"`
}

type AbsRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// Device is written to devices.json (path made absolute against the emu dir).
type Device struct {
	Name  string              `json:"name"`
	Path  string              `json:"path"`
	Props []string            `json:"props"`
	Keys  []string            `json:"keys"`
	Abs   map[string]AbsRange `json:"abs"`
}

// EInk holds approximate panel timings.
type EInk struct {
	// DurationsMS maps waveform name to update time in milliseconds.
	DurationsMS map[string]int `json:"durations_ms"`
	// FullFlashMS is the extra time of a full update (black/white flash).
	FullFlashMS int `json:"full_flash_ms"`
}

// Entry is one event of a template. Value is an integer or $id, $x, $y.
type Entry struct {
	Type  string `json:"type"`
	Code  string `json:"code"`
	Value string `json:"value"`
}

// Touch describes the Protocol A event sequence and the screen<->raw mapping.
type Touch struct {
	// Device-side raw->screen transform (the emulator applies its inverse).
	SwapXY  bool `json:"swap_xy"`
	MirrorX bool `json:"mirror_x"`
	MirrorY bool `json:"mirror_y"`

	MoveIntervalMS int `json:"move_interval_ms"`
	HoldIntervalMS int `json:"hold_interval_ms"`

	Press      []Entry `json:"press"`          // first group only, before contacts
	Contact    []Entry `json:"contact"`        // per finger, every group
	ReleasePer []Entry `json:"release_finger"` // per finger, in the release group
	Release    []Entry `json:"release"`        // once, in the release group
}

type Profile struct {
	Touch   Touch    `json:"touch"`
	FB      FBInfo   `json:"fb"`
	EInk    EInk     `json:"eink"`
	Devices []Device `json:"devices"`
}

func Load(path string) (*Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &p, p.Validate()
}

func (p *Profile) Validate() error {
	f := p.FB
	if f.BPP != 32 {
		return fmt.Errorf("fb: only 32 bpp supported, got %d", f.BPP)
	}
	if f.XRes <= 0 || f.YRes <= 0 {
		return fmt.Errorf("fb: invalid resolution %dx%d", f.XRes, f.YRes)
	}
	if f.LineLength < f.XRes*4 {
		return fmt.Errorf("fb: line_length %d < xres*4", f.LineLength)
	}
	if f.SmemLen < f.LineLength*f.YResVirtual {
		return fmt.Errorf("fb: smem_len %d < line_length*yres_virtual", f.SmemLen)
	}
	for _, c := range []Channel{f.Red, f.Green, f.Blue, f.Transp} {
		if c.Length != 8 || c.Offset%8 != 0 || c.Offset > 24 {
			return fmt.Errorf("fb: unsupported channel layout %+v", c)
		}
	}
	if len(p.Devices) == 0 {
		return fmt.Errorf("profile has no devices")
	}
	for _, d := range p.Devices {
		if d.Name == "" || d.Path == "" {
			return fmt.Errorf("device needs name and path")
		}
	}
	return nil
}
