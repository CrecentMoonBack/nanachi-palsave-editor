package gvas

import "fmt"

// gvasMagic is "GVAS" read as a little-endian int32.
const gvasMagic = 0x53415647

// CustomVersion is one entry of the engine's custom version table.
type CustomVersion struct {
	Key     GUID
	Version int32
}

// Header is the fixed preamble of a GVAS archive.
type Header struct {
	Magic                 int32
	SaveGameVersion       int32
	PackageFileVersionUE4 int32
	PackageFileVersionUE5 int32
	EngineVersionMajor    uint16
	EngineVersionMinor    uint16
	EngineVersionPatch    uint16
	EngineVersionChangeIt uint32
	EngineVersionBranch   String
	CustomVersionFormat   int32
	CustomVersions        []CustomVersion
	SaveGameClassName     String
}

// ReadHeader parses the archive preamble.
func ReadHeader(r *Reader) (*Header, error) {
	h := &Header{}
	var err error

	if h.Magic, err = r.I32(); err != nil {
		return nil, err
	}
	if h.Magic != gvasMagic {
		return nil, fmt.Errorf("%w: magic 0x%08x", ErrBadMagic, uint32(h.Magic))
	}
	if h.SaveGameVersion, err = r.I32(); err != nil {
		return nil, err
	}
	if h.SaveGameVersion != 3 {
		return nil, fmt.Errorf("%w: save game version %d, expected 3", ErrUnsupported, h.SaveGameVersion)
	}
	if h.PackageFileVersionUE4, err = r.I32(); err != nil {
		return nil, err
	}
	if h.PackageFileVersionUE5, err = r.I32(); err != nil {
		return nil, err
	}
	if h.EngineVersionMajor, err = r.U16(); err != nil {
		return nil, err
	}
	if h.EngineVersionMinor, err = r.U16(); err != nil {
		return nil, err
	}
	if h.EngineVersionPatch, err = r.U16(); err != nil {
		return nil, err
	}
	if h.EngineVersionChangeIt, err = r.U32(); err != nil {
		return nil, err
	}
	if h.EngineVersionBranch, err = r.String(); err != nil {
		return nil, err
	}
	if h.CustomVersionFormat, err = r.I32(); err != nil {
		return nil, err
	}
	if h.CustomVersionFormat != 3 {
		return nil, fmt.Errorf("%w: custom version format %d, expected 3", ErrUnsupported, h.CustomVersionFormat)
	}

	count, err := r.U32()
	if err != nil {
		return nil, err
	}
	h.CustomVersions = make([]CustomVersion, 0, count)
	for i := uint32(0); i < count; i++ {
		var cv CustomVersion
		if cv.Key, err = r.GUID(); err != nil {
			return nil, fmt.Errorf("custom version %d: %w", i, err)
		}
		if cv.Version, err = r.I32(); err != nil {
			return nil, fmt.Errorf("custom version %d: %w", i, err)
		}
		h.CustomVersions = append(h.CustomVersions, cv)
	}

	if h.SaveGameClassName, err = r.String(); err != nil {
		return nil, err
	}
	return h, nil
}

// Write serialises the preamble.
func (h *Header) Write(w *Writer) {
	w.I32(h.Magic)
	w.I32(h.SaveGameVersion)
	w.I32(h.PackageFileVersionUE4)
	w.I32(h.PackageFileVersionUE5)
	w.U16(h.EngineVersionMajor)
	w.U16(h.EngineVersionMinor)
	w.U16(h.EngineVersionPatch)
	w.U32(h.EngineVersionChangeIt)
	w.String(h.EngineVersionBranch)
	w.I32(h.CustomVersionFormat)
	w.U32(uint32(len(h.CustomVersions)))
	for _, cv := range h.CustomVersions {
		w.GUID(cv.Key)
		w.I32(cv.Version)
	}
	w.String(h.SaveGameClassName)
}

// EngineVersion renders the engine version the save was written by.
func (h *Header) EngineVersion() string {
	return fmt.Sprintf("%d.%d.%d-%d (%s)",
		h.EngineVersionMajor, h.EngineVersionMinor, h.EngineVersionPatch,
		h.EngineVersionChangeIt, h.EngineVersionBranch.Value)
}
