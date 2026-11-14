package touch

import (
	"encoding/json"
	"os"
	"syscall"
	"testing"
	"time"

	"kobo-emu/profile"
)

func load(t *testing.T) *profile.Profile {
	b, err := os.ReadFile("../profiles/clara-colour.json")
	if err != nil {
		t.Fatal(err)
	}
	var p profile.Profile
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func TestTransform(t *testing.T) {
	p := load(t)
	tr := NewTransform(p.Touch, 1072, 1448, 1447, 1071)
	// plan: hamX = ekranY, hamY = 1071 - ekranX
	if got := tr.ToRaw(Point{100, 700}); got != (Point{700, 971}) {
		t.Fatalf("got %+v", got)
	}
	if got := tr.ToRaw(Point{0, 0}); got != (Point{0, 1071}) {
		t.Fatalf("got %+v", got)
	}
}

func TestGroups(t *testing.T) {
	pr, err := NewProtocol(load(t).Touch)
	if err != nil {
		t.Fatal(err)
	}
	one := []Finger{{ID: 0, P: Point{10, 20}}}
	press := pr.Press(one)
	// BTN_TOOL_FINGER, BTN_TOUCH, 12-event contact, SYN_REPORT
	if len(press) != 15 || press[0].Code != 325 || press[1].Code != 330 || press[len(press)-1] != syn {
		t.Fatalf("press: %+v", press)
	}
	if press[2].Code != 57 || press[4].Value != 10 || press[5].Value != 20 || press[13].Code != 2 {
		t.Fatalf("contact block: %+v", press[2:14])
	}
	two := pr.Move([]Finger{{0, Point{1, 2}}, {1, Point{3, 4}}})
	if len(two) != 25 || two[0].Code != 57 || two[12].Code != 57 || two[12].Value != 1 {
		t.Fatalf("two-finger move: len %d", len(two))
	}
	rel := pr.Release(one)
	// ABS_X, ABS_Y, ABS_PRESSURE 0, BTN_TOUCH 0, BTN_TOOL_FINGER 0, SYN
	if len(rel) != 6 || rel[3].Code != 330 || rel[4].Code != 325 || rel[2].Value != 0 {
		t.Fatalf("release: %+v", rel)
	}
	if len(press[0].Bytes()) != 16 || Decode(press[5].Bytes()).Value != 20 {
		t.Fatal("encode")
	}
}

func TestControllerFIFO(t *testing.T) {
	p := load(t)
	path := t.TempDir() + "/event1"
	if err := CreateFIFO(path); err != nil {
		t.Fatal(err)
	}
	tr := NewTransform(p.Touch, 1072, 1448, 1447, 1071)
	c, err := NewController(p.Touch, tr, path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// no reader: gesture produces nothing and is reported
	c.SetFingers([]Finger{{0, Point{5, 5}}})
	time.Sleep(30 * time.Millisecond)
	c.SetFingers(nil)
	time.Sleep(30 * time.Millisecond)
	if r, _ := c.Status(); r {
		t.Fatal("reader reported without reader")
	}

	// with a reader: press ... release arrives, nothing stale before it
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c.SetFingers([]Finger{{0, Point{100, 700}}})
	time.Sleep(60 * time.Millisecond)
	c.SetFingers(nil)
	time.Sleep(60 * time.Millisecond)
	buf := make([]byte, 1<<16)
	n, _ := f.Read(buf)
	if n == 0 || n%16 != 0 {
		t.Fatalf("read %d bytes", n)
	}
	first, last := Decode(buf[:16]), Decode(buf[n-32:n-16])
	if first.Code != 325 || first.Value != 1 || last.Code != 325 || last.Value != 0 {
		t.Fatalf("first %+v last %+v", first, last)
	}
}
