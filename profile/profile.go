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

type Profile struct {
	FB      FBInfo   `json:"fb"`
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
