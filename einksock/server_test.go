package einksock

import (
	"bufio"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"kobo-emu/panel"
	"kobo-emu/profile"
)

func TestUpdateWait(t *testing.T) {
	info := profile.FBInfo{XRes: 100, YRes: 100, BPP: 32, LineLength: 400, SmemLen: 40000,
		Red: profile.Channel{Offset: 0, Length: 8}, Green: profile.Channel{Offset: 8, Length: 8}, Blue: profile.Channel{Offset: 16, Length: 8}}
	fb := make([]byte, info.SmemLen)
	for i := range fb {
		fb[i] = 0xFF
	}
	conf := profile.EInk{DurationsMS: map[string]int{"GC16": 150}}
	pn := panel.New(info, fb, panel.Approx, conf)
	srv, err := Listen(filepath.Join(t.TempDir(), "s"), pn)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	c, err := net.Dial("unix", srv.ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	rd := bufio.NewReader(c)
	px := func(x, y int) byte {
		b := make([]byte, 100*100*4)
		pn.Snapshot(b)
		return b[(y*100+x)*4]
	}

	// black square in fb, no update yet: panel stays white
	for y := 10; y < 20; y++ {
		for x := 10; x < 20; x++ {
			p := y*400 + x*4
			fb[p], fb[p+1], fb[p+2] = 0, 0, 0
		}
	}
	if px(15, 15) != 0xFF {
		t.Fatal("panel changed without update")
	}
	fmt.Fprintln(c, `{"op":"update","marker":7,"rect":[10,10,10,10],"waveform":"GC16","full":false}`)
	if l, _ := rd.ReadString('\n'); l != "{\"op\":\"queued\",\"marker\":7}\n" {
		t.Fatalf("got %q", l)
	}
	if px(15, 15) != 0 {
		t.Fatal("rect not copied")
	}
	if px(25, 25) != 0xFF {
		t.Fatal("outside rect changed")
	}
	start := time.Now()
	fmt.Fprintln(c, `{"op":"wait","marker":7}`)
	if l, _ := rd.ReadString('\n'); l != "{\"op\":\"done\",\"marker\":7}\n" {
		t.Fatalf("got %q", l)
	}
	if d := time.Since(start); d < 100*time.Millisecond {
		t.Fatalf("wait returned too early: %v", d)
	}
}
