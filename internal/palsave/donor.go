package palsave

import (
	_ "embed"
	"fmt"
)

// donorTemplate is one ordinary pal's record, kept so a pal can be added to a
// save that holds none of its own — a brand-new single-player world starts with
// an empty CharacterSaveParameterMap and there is nothing to clone.
//
// It was lifted from a real save and stripped of everything identifying: the
// owner and previous-owner ids are zeroed and the nickname, last-jumped location
// and slot are gone. What remains is the neutral skeleton the game accepts;
// preparePal overwrites the species, owner, level and the rest on top of it.
//
//go:embed data/donor_pal.bin
var donorTemplate []byte

// slotVersionData is a pal container slot's version stamp. appendSlot copies it
// from an occupied slot, but the first pal added to a brand-new box has none to
// copy from, so this is the fallback. Like donorTemplate it is a game-version
// constant lifted from a real save; a game update could require refreshing it.
var slotVersionData = []byte{
	0x01, 0x00, 0x00, 0x00, 0x38, 0x0b, 0x00, 0xde, 0x49, 0x49, 0xd7, 0xce,
	0x97, 0xdf, 0x2d, 0x99, 0xc0, 0xc1, 0xc3, 0x69, 0x01, 0x00, 0x00, 0x00,
}

// slotTrailerLen is the number of trailing bytes a container slot carries after
// its two GUIDs and permission byte. Observed as five zero bytes; used to build
// a slot when there is no occupied one to copy the trailer from.
const slotTrailerLen = 5

// templateDonor decodes the embedded template into a fresh pal record.
func (w *World) templateDonor() (*Pal, error) {
	raw, err := DecodeCharacter(donorTemplate, w.opts)
	if err != nil {
		return nil, fmt.Errorf("palsave: decoding the embedded donor: %w", err)
	}
	return NewPal(raw)
}

// donorRecord returns a pal record to clone: an ordinary pal already in the
// save when there is one, otherwise the embedded template. The in-save donor is
// preferred because it carries the exact version stamp this save was written
// with; the template is the fallback that makes an empty world workable.
func (w *World) donorRecord() (*Pal, error) {
	if c, ok := w.donorPal(); ok {
		return c.Pal, nil
	}
	return w.templateDonor()
}
