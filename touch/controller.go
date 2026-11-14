package touch

import (
	"os"
	"sync"
	"time"

	"kobo-emu/profile"
)

// Controller turns the current finger positions (screen pixels) into Kobo
// event groups with the device's timing. Feed it with SetFingers.
type Controller struct {
	proto *Protocol
	tr    Transform
	w     *Writer
	conf  profile.Touch

	mu      sync.Mutex
	fingers []Finger // screen coordinates, ID = finger index
	reader  bool
	rec     *os.File

	stop chan struct{}
}

func NewController(conf profile.Touch, tr Transform, fifoPath, recordPath string) (*Controller, error) {
	pr, err := NewProtocol(conf)
	if err != nil {
		return nil, err
	}
	c := &Controller{proto: pr, tr: tr, w: NewWriter(fifoPath), conf: conf, stop: make(chan struct{})}
	if recordPath != "" {
		if c.rec, err = os.Create(recordPath); err != nil {
			return nil, err
		}
	}
	go c.loop()
	return c, nil
}

func (c *Controller) Close() {
	close(c.stop)
	if c.rec != nil {
		c.rec.Close()
	}
}

// SetFingers sets the current contacts in screen pixels (empty = lifted).
func (c *Controller) SetFingers(fs []Finger) {
	c.mu.Lock()
	c.fingers = append(c.fingers[:0], fs...)
	c.mu.Unlock()
}

// Status reports whether a reader was attached at the last gesture start.
func (c *Controller) Status() (reader bool, dropped int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reader, c.w.Dropped
}

func (c *Controller) raw(fs []Finger) []Finger {
	out := make([]Finger, len(fs))
	for i, f := range fs {
		out[i] = Finger{ID: f.ID, P: c.tr.ToRaw(f.P)}
	}
	return out
}

func equal(a, b []Finger) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (c *Controller) emit(g []Event, send bool) {
	now := time.Now()
	for i := range g {
		g[i].Sec, g[i].Usec = uint32(now.Unix()), uint32(now.Nanosecond()/1000)
	}
	if c.rec != nil {
		for _, e := range g {
			c.rec.Write(e.Bytes())
		}
	}
	if send {
		c.mu.Lock()
		c.w.WriteGroup(g)
		c.mu.Unlock()
	}
}

func (c *Controller) loop() {
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()
	var (
		active   bool
		send     bool
		last     []Finger
		lastSent time.Time
	)
	moveIv := time.Duration(c.conf.MoveIntervalMS) * time.Millisecond
	holdIv := time.Duration(c.conf.HoldIntervalMS) * time.Millisecond
	for {
		select {
		case <-c.stop:
			return
		case <-tick.C:
		}
		c.mu.Lock()
		cur := c.raw(c.fingers)
		c.mu.Unlock()

		switch {
		case !active && len(cur) > 0:
			active = true
			c.mu.Lock()
			send = c.w.HasReader()
			c.reader = send
			c.mu.Unlock()
			c.emit(c.proto.Press(cur), send)
			last, lastSent = cur, time.Now()
		case active && len(cur) == 0:
			c.emit(c.proto.Release(last), send)
			active, last = false, nil
		case active:
			since := time.Since(lastSent)
			moved := !equal(cur, last)
			if (moved && since >= moveIv) || (!moved && since >= holdIv) {
				c.emit(c.proto.Move(cur), send)
				last, lastSent = cur, time.Now()
			}
		}
	}
}
