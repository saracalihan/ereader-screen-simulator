package gfx

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

var wfNames = map[Waveform]string{INIT: "INIT", DU: "DU", GC16: "GC16", GL16: "GL16", GLR16: "GLR16", A2: "A2"}

// dialEmu returns a refresh func that talks to kobo-emu's eink.sock, or nil
// when the emulator is not listening (drawing then simply stays unrefreshed).
func dialEmu(path string) func(x, y, w, h int, wf Waveform, full bool) error {
	var (
		mu     sync.Mutex
		conn   net.Conn
		rd     *bufio.Reader
		marker uint32
	)
	connect := func() error {
		c, err := net.Dial("unix", path)
		if err != nil {
			return err
		}
		conn, rd = c, bufio.NewReader(c)
		return nil
	}
	if connect() != nil {
		return nil
	}
	type resp struct {
		Op     string `json:"op"`
		Marker uint32 `json:"marker"`
		Msg    string `json:"msg"`
	}
	roundtrip := func(req map[string]any, want string) error {
		b, _ := json.Marshal(req)
		if _, err := conn.Write(append(b, '\n')); err != nil {
			return err
		}
		line, err := rd.ReadBytes('\n')
		if err != nil {
			return err
		}
		var r resp
		if err := json.Unmarshal(line, &r); err != nil {
			return err
		}
		if r.Op == "error" {
			return fmt.Errorf("eink.sock: %s", r.Msg)
		}
		if r.Op != want {
			return fmt.Errorf("eink.sock: got %q, want %q", r.Op, want)
		}
		return nil
	}
	return func(x, y, w, h int, wf Waveform, full bool) error {
		mu.Lock()
		defer mu.Unlock()
		name, ok := wfNames[wf]
		if !ok {
			name = "GC16"
		}
		marker++
		var err error
		for try := 0; try < 2; try++ { // one reconnect if the emulator restarted
			if conn == nil {
				if err = connect(); err != nil {
					return err
				}
			}
			err = roundtrip(map[string]any{"op": "update", "marker": marker, "rect": []int{x, y, w, h}, "waveform": name, "full": full}, "queued")
			if err == nil {
				err = roundtrip(map[string]any{"op": "wait", "marker": marker}, "done")
			}
			if err == nil {
				return nil
			}
			conn.Close()
			conn = nil
		}
		return err
	}
}
