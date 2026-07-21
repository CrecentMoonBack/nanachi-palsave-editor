//go:build windows

package oodle

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Codec ids accepted by NanachiOozCompress, from ooz's compress.h.
const (
	CodecKraken    = 8
	CodecMermaid   = 9
	CodecSelkie    = 11
	CodecLeviathan = 13
)

// Compression levels. Palworld saves are written with Kraken at Normal, which
// is what palooz uses; keep that default so output stays consistent with the
// reference implementation.
const (
	LevelSuperFast = 1
	LevelVeryFast  = 2
	LevelFast      = 3
	LevelNormal    = 4
	LevelOptimal1  = 5
)

// Scratch space the native side expects the caller to provide.
const (
	decompressPadding = 64
	compressPadding   = 65536
)

const dllName = "nanachi_ooz.dll"

var (
	loadOnce       sync.Once
	loadErr        error
	dll            *windows.LazyDLL
	procDecompress *windows.LazyProc
	procCompress   *windows.LazyProc
)

// dllSearchPaths returns the places we look for the native codec, in order:
// alongside the executable first (how a release is laid out), then the
// in-repo build output (how `go test` and `wails dev` see it).
func dllSearchPaths() []string {
	var paths []string
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), dllName))
	}
	if wd, err := os.Getwd(); err == nil {
		paths = append(paths,
			filepath.Join(wd, dllName),
			filepath.Join(wd, "third_party", "oodle", dllName),
			// from internal/oodle during `go test ./...`
			filepath.Join(wd, "..", "..", "third_party", "oodle", dllName),
		)
	}
	return paths
}

func load() error {
	loadOnce.Do(func() {
		var found string
		for _, p := range dllSearchPaths() {
			if _, err := os.Stat(p); err == nil {
				found = p
				break
			}
		}
		if found == "" {
			loadErr = fmt.Errorf("oodle: %s not found; build it with third_party/oodle/build.sh", dllName)
			return
		}
		abs, err := filepath.Abs(found)
		if err != nil {
			loadErr = fmt.Errorf("oodle: resolving %s: %w", found, err)
			return
		}
		dll = windows.NewLazyDLL(abs)
		if err := dll.Load(); err != nil {
			loadErr = fmt.Errorf("oodle: loading %s: %w", abs, err)
			return
		}
		procDecompress = dll.NewProc("NanachiOozDecompress")
		procCompress = dll.NewProc("NanachiOozCompress")
		if err := procDecompress.Find(); err != nil {
			loadErr = fmt.Errorf("oodle: NanachiOozDecompress missing from %s: %w", abs, err)
			return
		}
		if err := procCompress.Find(); err != nil {
			loadErr = fmt.Errorf("oodle: NanachiOozCompress missing from %s: %w", abs, err)
			return
		}
	})
	return loadErr
}

// Available reports whether the native codec could be loaded. Useful for
// surfacing a clear message in the UI instead of failing at the first save.
func Available() error { return load() }

// rawDecompress runs a Kraken stream through the native decoder.
func rawDecompress(src []byte, uncompressedLen int) ([]byte, error) {
	if err := load(); err != nil {
		return nil, err
	}
	if len(src) == 0 || uncompressedLen <= 0 {
		return nil, fmt.Errorf("oodle: nothing to decompress")
	}
	dst := make([]byte, uncompressedLen+decompressPadding)

	r, _, _ := procDecompress.Call(
		uintptr(unsafe.Pointer(&src[0])),
		uintptr(len(src)),
		uintptr(unsafe.Pointer(&dst[0])),
		uintptr(uncompressedLen),
	)
	runtime.KeepAlive(src)
	runtime.KeepAlive(dst)

	n := int(int64(r))
	if n < 0 {
		return nil, fmt.Errorf("oodle: decompress failed (code %d)", n)
	}
	if n != uncompressedLen {
		return nil, fmt.Errorf("%w: got %d bytes, header said %d", ErrSizeMismatch, n, uncompressedLen)
	}
	return dst[:n], nil
}

// rawCompress runs data through the native Kraken encoder.
func rawCompress(src []byte, codec, level int) ([]byte, error) {
	if err := load(); err != nil {
		return nil, err
	}
	if len(src) == 0 {
		return nil, fmt.Errorf("oodle: nothing to compress")
	}
	dst := make([]byte, len(src)+compressPadding)

	r, _, _ := procCompress.Call(
		uintptr(codec),
		uintptr(level),
		uintptr(unsafe.Pointer(&src[0])),
		uintptr(len(src)),
		uintptr(unsafe.Pointer(&dst[0])),
		uintptr(len(dst)),
	)
	runtime.KeepAlive(src)
	runtime.KeepAlive(dst)

	n := int(int64(r))
	if n < 0 {
		return nil, fmt.Errorf("oodle: compress failed (code %d)", n)
	}
	if n > len(dst) {
		return nil, fmt.Errorf("oodle: compressor reported %d bytes into a %d byte buffer", n, len(dst))
	}
	return dst[:n], nil
}

// DecompressSav turns a whole .sav file into its raw GVAS bytes, reporting the
// save type so a later Compress can round-trip the same container.
func DecompressSav(data []byte) ([]byte, SaveType, error) {
	h, err := ParseHeader(data)
	if err != nil {
		return nil, 0, err
	}
	payload := h.Payload(data)

	switch h.Type {
	case SaveTypePLM:
		out, err := rawDecompress(payload, int(h.UncompressedLen))
		return out, h.Type, err

	case SaveTypePLZ, SaveTypeCNK:
		zr, err := zlib.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, h.Type, fmt.Errorf("oodle: zlib reader: %w", err)
		}
		defer zr.Close()
		out, err := io.ReadAll(zr)
		if err != nil {
			return nil, h.Type, fmt.Errorf("oodle: zlib decompress: %w", err)
		}
		if len(out) != int(h.UncompressedLen) {
			return nil, h.Type, fmt.Errorf("%w: got %d bytes, header said %d",
				ErrSizeMismatch, len(out), h.UncompressedLen)
		}
		return out, h.Type, nil

	default:
		return nil, h.Type, fmt.Errorf("%w: %v", ErrUnknownMagic, h.Type)
	}
}

// CompressSav packs raw GVAS bytes back into a .sav container.
//
// The output is not byte-identical to what the game writes: ooz's Kraken
// encoder is not RAD's, and in practice produces a noticeably smaller file for
// the same data. Verify edits by comparing entity counts, never file size.
func CompressSav(gvas []byte, t SaveType) ([]byte, error) {
	switch t {
	case SaveTypePLM:
		packed, err := rawCompress(gvas, CodecKraken, LevelNormal)
		if err != nil {
			return nil, err
		}
		return BuildSav(packed, len(gvas), t)

	case SaveTypePLZ, SaveTypeCNK:
		var buf bytes.Buffer
		zw := zlib.NewWriter(&buf)
		if _, err := zw.Write(gvas); err != nil {
			return nil, fmt.Errorf("oodle: zlib write: %w", err)
		}
		if err := zw.Close(); err != nil {
			return nil, fmt.Errorf("oodle: zlib close: %w", err)
		}
		return BuildSav(buf.Bytes(), len(gvas), t)

	default:
		return nil, fmt.Errorf("%w: %v", ErrUnknownMagic, t)
	}
}
