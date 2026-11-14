package touch

import (
	"fmt"
	"strconv"

	"kobo-emu/profile"
)

var types = map[string]uint16{"EV_SYN": 0, "EV_KEY": 1, "EV_ABS": 3}

var codes = map[string]uint16{
	"SYN_REPORT": 0, "SYN_MT_REPORT": 2,
	"BTN_TOUCH": 330, "BTN_TOOL_FINGER": 325, "BTN_TOOL_PEN": 320,
	"ABS_X": 0, "ABS_Y": 1, "ABS_PRESSURE": 24,
	"ABS_MT_TOUCH_MAJOR": 48, "ABS_MT_TOUCH_MINOR": 49, "ABS_MT_ORIENTATION": 52,
	"ABS_MT_POSITION_X": 53, "ABS_MT_POSITION_Y": 54, "ABS_MT_TOOL_TYPE": 55,
	"ABS_MT_TRACKING_ID": 57, "ABS_MT_PRESSURE": 58, "ABS_MT_DISTANCE": 59,
}

type valKind int

const (
	vConst valKind = iota
	vID
	vX
	vY
)

type tmpl struct {
	typ, code uint16
	kind      valKind
	val       int32
}

// Finger is one contact: Id is the finger index, P the raw coordinate.
type Finger struct {
	ID int
	P  Point
}

// Protocol builds event groups from the profile's templates.
type Protocol struct {
	press, contact, relPer, release []tmpl
}

func compile(es []profile.Entry) ([]tmpl, error) {
	out := make([]tmpl, 0, len(es))
	for _, e := range es {
		t, ok := types[e.Type]
		if !ok {
			return nil, fmt.Errorf("unknown event type %q", e.Type)
		}
		c, ok := codes[e.Code]
		if !ok {
			return nil, fmt.Errorf("unknown event code %q", e.Code)
		}
		x := tmpl{typ: t, code: c}
		switch e.Value {
		case "$id":
			x.kind = vID
		case "$x":
			x.kind = vX
		case "$y":
			x.kind = vY
		default:
			n, err := strconv.Atoi(e.Value)
			if err != nil {
				return nil, fmt.Errorf("bad value %q for %s", e.Value, e.Code)
			}
			x.val = int32(n)
		}
		out = append(out, x)
	}
	return out, nil
}

func NewProtocol(t profile.Touch) (*Protocol, error) {
	p := &Protocol{}
	var err error
	for _, s := range []struct {
		dst *[]tmpl
		src []profile.Entry
	}{{&p.press, t.Press}, {&p.contact, t.Contact}, {&p.relPer, t.ReleasePer}, {&p.release, t.Release}} {
		if *s.dst, err = compile(s.src); err != nil {
			return nil, err
		}
	}
	if len(p.contact) == 0 {
		return nil, fmt.Errorf("touch.contact template is empty")
	}
	return p, nil
}

func expand(dst []Event, ts []tmpl, f Finger) []Event {
	for _, t := range ts {
		v := t.val
		switch t.kind {
		case vID:
			v = int32(f.ID)
		case vX:
			v = int32(f.P.X)
		case vY:
			v = int32(f.P.Y)
		}
		dst = append(dst, Event{Type: t.typ, Code: t.code, Value: v})
	}
	return dst
}

var syn = Event{Type: 0, Code: 0, Value: 0}

// Press is the first group of a gesture.
func (p *Protocol) Press(fs []Finger) []Event {
	g := expand(nil, p.press, Finger{})
	for _, f := range fs {
		g = expand(g, p.contact, f)
	}
	return append(g, syn)
}

// Move is a continuation group (dragging or holding).
func (p *Protocol) Move(fs []Finger) []Event {
	var g []Event
	for _, f := range fs {
		g = expand(g, p.contact, f)
	}
	return append(g, syn)
}

// Release is the last group; fs are the fingers of the previous group.
func (p *Protocol) Release(fs []Finger) []Event {
	var g []Event
	for _, f := range fs {
		g = expand(g, p.relPer, f)
	}
	g = expand(g, p.release, Finger{})
	return append(g, syn)
}
