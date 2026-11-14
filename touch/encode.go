package touch

import "encoding/binary"

// Event is a Linux input_event as laid out on the Kobo (32-bit timeval, 16 bytes).
type Event struct {
	Sec, Usec  uint32
	Type, Code uint16
	Value      int32
}

const EventSize = 16

func (e Event) Bytes() []byte {
	b := make([]byte, EventSize)
	binary.LittleEndian.PutUint32(b[0:], e.Sec)
	binary.LittleEndian.PutUint32(b[4:], e.Usec)
	binary.LittleEndian.PutUint16(b[8:], e.Type)
	binary.LittleEndian.PutUint16(b[10:], e.Code)
	binary.LittleEndian.PutUint32(b[12:], uint32(e.Value))
	return b
}

func Decode(b []byte) Event {
	return Event{
		Sec: binary.LittleEndian.Uint32(b[0:]), Usec: binary.LittleEndian.Uint32(b[4:]),
		Type: binary.LittleEndian.Uint16(b[8:]), Code: binary.LittleEndian.Uint16(b[10:]),
		Value: int32(binary.LittleEndian.Uint32(b[12:])),
	}
}
