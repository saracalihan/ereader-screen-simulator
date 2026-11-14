// kobo-emu: Kobo screen + touch emulator. contract files + raw fb window.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"kobo-emu/einksock"
	"kobo-emu/fbfile"
	"kobo-emu/panel"
	"kobo-emu/profile"
	"kobo-emu/touch"
	"kobo-emu/ui"
)

func main() {
	dir := flag.String("dir", "/tmp/kobo-emu", "contract files directory")
	prof := flag.String("profile", "profiles/clara-colour.json", "device profile")
	scale := flag.Float64("scale", 0, "window scale (0 = fit monitor height)")
	einkMode := flag.String("eink", "ideal", "ideal | approx | raw (raw = start in raw fb view)")
	record := flag.String("record", "", "write generated touch events to this file")
	flag.Parse()

	p, err := profile.Load(*prof)
	if err != nil {
		log.Fatal(err)
	}
	abs, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		log.Fatal(err)
	}

	fb, err := fbfile.Create(abs, p.FB)
	if err != nil {
		log.Fatal(err)
	}
	defer fb.Close()

	devs := make([]profile.Device, len(p.Devices))
	copy(devs, p.Devices)
	for i := range devs {
		devs[i].Path = filepath.Join(abs, devs[i].Path)
		if err := touch.CreateFIFO(devs[i].Path); err != nil {
			log.Fatal(err)
		}
	}
	if err := fbfile.WriteJSON(filepath.Join(abs, "devices.json"), devs); err != nil {
		log.Fatal(err)
	}

	pm, err := panel.ParseMode(*einkMode)
	if err != nil {
		log.Fatal(err)
	}
	pn := panel.New(p.FB, fb.Buf, pm, p.EInk)
	srv, err := einksock.Listen(filepath.Join(abs, "eink.sock"), pn)
	if err != nil {
		log.Fatal(err)
	}
	defer srv.Close()

	rawMax := func(axis string) int { return devs[0].Abs[axis].Max }
	tr := touch.NewTransform(p.Touch, p.FB.XRes, p.FB.YRes, rawMax("ABS_MT_POSITION_X"), rawMax("ABS_MT_POSITION_Y"))
	tc, err := touch.NewController(p.Touch, tr, devs[0].Path, *record)
	if err != nil {
		log.Fatal(err)
	}
	defer tc.Close()

	fmt.Printf("kobo-emu: contract files in %s\n", abs)
	for _, n := range []string{"fb0", "fb0.json", "devices.json", "eink.sock"} {
		fmt.Println("  ", filepath.Join(abs, n))
	}
	for _, d := range devs {
		fmt.Println("  ", d.Path)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() { <-c; os.Exit(0) }()

	if err := ui.Run(p.FB, fb.Buf, pn, tc, *scale, *einkMode == "raw"); err != nil {
		log.Fatal(err)
	}
}
