// kobo-emu: Kobo screen + touch emulator. contract files + raw fb window.
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"kobo-emu/fbfile"
	"kobo-emu/profile"
	"kobo-emu/touch"
	"kobo-emu/ui"
)

func main() {
	dir := flag.String("dir", "/tmp/kobo-emu", "contract files directory")
	prof := flag.String("profile", "profiles/clara-colour.json", "device profile")
	scale := flag.Float64("scale", 0, "window scale (0 = fit monitor height)")
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

	// eink.sock: created now so the path exists; protocol arrives in step 3.
	sock := filepath.Join(abs, "eink.sock")
	os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

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

	if err := ui.Run(p.FB, fb.Buf, *scale); err != nil {
		log.Fatal(err)
	}
}
