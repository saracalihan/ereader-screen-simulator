// Package einksock serves the eink.sock update/wait protocol (JSON lines).
package einksock

import (
	"bufio"
	"encoding/json"
	"image"
	"net"
	"os"
	"sync"
	"time"

	"kobo-emu/panel"
)

type request struct {
	Op       string `json:"op"`
	Marker   uint32 `json:"marker"`
	Rect     []int  `json:"rect"`
	Waveform string `json:"waveform"`
	Full     bool   `json:"full"`
}

type response struct {
	Op     string `json:"op"`
	Marker uint32 `json:"marker"`
	Msg    string `json:"msg,omitempty"`
}

type Server struct {
	Panel *panel.Panel
	ln    net.Listener

	mu      sync.Mutex
	pending map[uint32]chan struct{} // closed when the update is done
}

func Listen(path string, p *panel.Panel) (*Server, error) {
	os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	os.Chmod(path, 0o666)
	s := &Server{Panel: p, ln: ln, pending: map[uint32]chan struct{}{}}
	go s.accept()
	return s, nil
}

func (s *Server) Close() error { return s.ln.Close() }

func (s *Server) accept() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.serve(c)
	}
}

func (s *Server) serve(c net.Conn) {
	defer c.Close()
	var wmu sync.Mutex
	send := func(r response) {
		b, _ := json.Marshal(r)
		wmu.Lock()
		c.Write(append(b, '\n'))
		wmu.Unlock()
	}
	sc := bufio.NewScanner(c)
	for sc.Scan() {
		var rq request
		if err := json.Unmarshal(sc.Bytes(), &rq); err != nil {
			send(response{Op: "error", Msg: "bad json: " + err.Error()})
			continue
		}
		switch rq.Op {
		case "update":
			if len(rq.Rect) != 4 {
				send(response{Op: "error", Marker: rq.Marker, Msg: "rect must be [x,y,w,h]"})
				continue
			}
			r := image.Rect(rq.Rect[0], rq.Rect[1], rq.Rect[0]+rq.Rect[2], rq.Rect[1]+rq.Rect[3])
			dur, err := s.Panel.Update(r, rq.Waveform, rq.Full)
			if err != nil {
				send(response{Op: "error", Marker: rq.Marker, Msg: err.Error()})
				continue
			}
			done := make(chan struct{})
			s.mu.Lock()
			s.pending[rq.Marker] = done
			s.mu.Unlock()
			time.AfterFunc(dur, func() {
				close(done)
				s.mu.Lock()
				if s.pending[rq.Marker] == done {
					delete(s.pending, rq.Marker)
				}
				s.mu.Unlock()
			})
			send(response{Op: "queued", Marker: rq.Marker})
		case "wait":
			s.mu.Lock()
			done := s.pending[rq.Marker]
			s.mu.Unlock()
			go func(m uint32) { // don't block other requests on this conn
				if done != nil {
					<-done
				}
				send(response{Op: "done", Marker: m})
			}(rq.Marker)
		default:
			send(response{Op: "error", Marker: rq.Marker, Msg: "unknown op " + rq.Op})
		}
	}
}
