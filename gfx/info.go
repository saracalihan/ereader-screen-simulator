package gfx

// fbInfo mirrors the fb0.json contract (and what the ioctls return on the device).
type fbInfo struct {
	XRes       int `json:"xres"`
	YRes       int `json:"yres"`
	BPP        int `json:"bits_per_pixel"`
	LineLength int `json:"line_length"`
	SmemLen    int `json:"smem_len"`
	Red        chn `json:"red"`
	Green      chn `json:"green"`
	Blue       chn `json:"blue"`
	Transp     chn `json:"transp"`
}

type chn struct {
	Offset int `json:"offset"`
	Length int `json:"length"`
}
