package palsave

import (
	"encoding/binary"
	"unicode/utf16"

	"github.com/CrecentMoonBack/nanachi-palsave-editor/internal/gvas"
)

// Base camps and guilds live in RawData blobs rather than in the property
// tree, so they need reading by hand.
//
// Only the leading fields of each blob are read — enough to answer "which
// camps does this guild own, and which container holds each one's workers".
// Stopping early is deliberate: the tail of a guild blob has changed shape
// between game versions (the 2026-07 update inserted chest roles and
// permissions before the player list), and nothing here needs it. Everything
// read below sits ahead of any known variation.
//
// Layouts follow palsav's rawdata readers, which is also where the field names
// come from.

// blobReader walks one of those little-endian blobs.
//
// A short or malformed blob sets bad and every later read returns a zero
// value, so a caller can do the whole sequence and check once at the end
// rather than after every field. Half a camp is not worth reporting.
type blobReader struct {
	b   []byte
	off int
	bad bool
}

func (r *blobReader) need(n int) bool {
	if r.bad || r.off+n > len(r.b) {
		r.bad = true
		return false
	}
	return true
}

func (r *blobReader) skip(n int) {
	if r.need(n) {
		r.off += n
	}
}

func (r *blobReader) u32() uint32 {
	if !r.need(4) {
		return 0
	}
	v := binary.LittleEndian.Uint32(r.b[r.off:])
	r.off += 4
	return v
}

func (r *blobReader) guid() gvas.GUID {
	var g gvas.GUID
	if !r.need(16) {
		return g
	}
	copy(g[:], r.b[r.off:r.off+16])
	r.off += 16
	return g
}

// fstring reads an Unreal string: a length, then bytes. A negative length
// counts UTF-16 code units instead of bytes. Both forms include a trailing
// null that is not part of the text.
func (r *blobReader) fstring() string {
	n := int(int32(r.u32()))
	switch {
	case r.bad || n == 0:
		return ""
	case n < 0:
		n = -n
		if !r.need(n * 2) {
			return ""
		}
		u := make([]uint16, n)
		for i := range u {
			u[i] = binary.LittleEndian.Uint16(r.b[r.off+i*2:])
		}
		r.off += n * 2
		return string(utf16.Decode(u[:n-1]))
	default:
		if !r.need(n) {
			return ""
		}
		s := string(r.b[r.off : r.off+n-1])
		r.off += n
		return s
	}
}

// Sizes of the fields skipped over rather than decoded.
const (
	// FTransform is a quaternion plus two vectors, all doubles.
	sizeTransform = 4*8 + 3*8 + 3*8
	// One entry of individual_character_handle_ids: owner uid, instance id.
	sizeCharacterHandle = 16 + 16
)

// BaseCamp is one of a guild's camps.
type BaseCamp struct {
	ID   gvas.GUID
	Name string
	// GuildID is the group the camp belongs to.
	GuildID gvas.GUID
	// ContainerID holds the camp's working pals.
	ContainerID gvas.GUID
}

// Guild is a group of players, with the members needed to match a player to it.
//
// No display name: the blob's group_name field holds the leader's UID, not
// anything worth showing, and the real guild_name sits past the version-varying
// part nothing here reads.
type Guild struct {
	ID gvas.GUID
	// Members are the player UIDs whose characters this group owns.
	Members map[gvas.GUID]bool
}

// BaseCamps lists every camp in the save, of every guild.
//
// Read from BaseCampSaveData rather than inferred from where pals are, so a
// camp with no workers still appears — it exists on the map either way.
func (w *World) BaseCamps() []BaseCamp {
	m, err := w.mapProp("BaseCampSaveData")
	if err != nil {
		return nil
	}
	out := make([]BaseCamp, 0, len(m.Entries))
	for i := range m.Entries {
		props, ok := m.Entries[i].Value.Struct.(*gvas.StructProperties)
		if !ok {
			continue
		}
		camp, ok := readBaseCamp(props.Props)
		if !ok {
			continue
		}
		out = append(out, camp)
	}
	return out
}

func readBaseCamp(props *gvas.Properties) (BaseCamp, bool) {
	raw, ok := rawDataArray(props)
	if !ok {
		return BaseCamp{}, false
	}
	r := &blobReader{b: raw.Values.Bytes}
	camp := BaseCamp{ID: r.guid(), Name: r.fstring()}
	r.skip(1)             // state
	r.skip(sizeTransform) // transform
	r.skip(4)             // area_range
	camp.GuildID = r.guid()
	if r.bad {
		return BaseCamp{}, false
	}

	// The worker container is in a nested blob of its own.
	v, ok := props.Get("WorkerDirector")
	if !ok {
		return BaseCamp{}, false
	}
	sp, ok := v.(*gvas.StructProperty)
	if !ok {
		return BaseCamp{}, false
	}
	inner, ok := sp.Value.(*gvas.StructProperties)
	if !ok {
		return BaseCamp{}, false
	}
	wraw, ok := rawDataArray(inner.Props)
	if !ok {
		return BaseCamp{}, false
	}
	wr := &blobReader{b: wraw.Values.Bytes}
	wr.skip(16)            // id
	wr.skip(sizeTransform) // spawn_transform
	wr.skip(2)             // current_order_type, current_battle_type
	camp.ContainerID = wr.guid()
	if wr.bad {
		return BaseCamp{}, false
	}
	return camp, true
}

// Guilds lists the groups in the save.
//
// Only the group id, name and membership are read. Membership comes from
// individual_character_handle_ids, whose entries pair an owning player UID
// with one of their characters — every member appears there because a player's
// own character is one of them.
func (w *World) Guilds() []Guild {
	m, err := w.mapProp("GroupSaveDataMap")
	if err != nil {
		return nil
	}
	out := make([]Guild, 0, len(m.Entries))
	for i := range m.Entries {
		props, ok := m.Entries[i].Value.Struct.(*gvas.StructProperties)
		if !ok {
			continue
		}
		raw, ok := rawDataArray(props.Props)
		if !ok {
			continue
		}
		r := &blobReader{b: raw.Values.Bytes}
		g := Guild{ID: r.guid(), Members: map[gvas.GUID]bool{}}
		r.fstring() // group_name: the leader's UID, not a display name
		count := int(r.u32())
		for j := 0; j < count && !r.bad; j++ {
			g.Members[r.guid()] = true // owner player uid
			r.skip(sizeCharacterHandle - 16)
		}
		if r.bad {
			continue
		}
		out = append(out, g)
	}
	return out
}

// GuildOf returns the group a player belongs to.
func (w *World) GuildOf(player gvas.GUID) (Guild, bool) {
	for _, g := range w.Guilds() {
		if g.Members[player] {
			return g, true
		}
	}
	return Guild{}, false
}
