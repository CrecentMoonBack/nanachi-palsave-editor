// Command palsave is the CLI face of the editor.
//
// Right now it only reports what it can see, which is enough to confirm the
// native codec resolves in whatever directory layout it was shipped in.
package main

import (
	"fmt"
	"os"

	"github.com/wo420/nanachi-palsave-editor/internal/oodle"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		fmt.Println("usage: palsave info <Level.sav>")
		fmt.Println("       palsave codec")
		return nil
	}

	switch os.Args[1] {
	case "codec":
		if err := oodle.Available(); err != nil {
			return err
		}
		fmt.Println("oodle codec: ok")
		return nil

	case "info":
		if len(os.Args) < 3 {
			return fmt.Errorf("info needs a path to a .sav file")
		}
		return info(os.Args[2])

	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}

func info(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	h, err := oodle.ParseHeader(data)
	if err != nil {
		return err
	}
	fmt.Printf("file        %s\n", path)
	fmt.Printf("size        %d bytes\n", len(data))
	fmt.Printf("format      %s (save_type %d)\n", h.Type, byte(h.Type))
	fmt.Printf("compressed  %d bytes\n", h.CompressedLen)
	fmt.Printf("expanded    %d bytes\n", h.UncompressedLen)

	gvas, _, err := oodle.DecompressSav(data)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}
	fmt.Printf("decompressed %d bytes, magic %q\n", len(gvas), gvas[:4])
	return nil
}
