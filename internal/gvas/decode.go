package gvas

import "fmt"

// Options controls how the tree is decoded.
type Options struct {
	// TypeHints maps a property path to the struct type its elements use,
	// because map keys and values do not always name their own struct type.
	TypeHints map[string]string
}

func (o *Options) hint(path, fallback string) string {
	if o != nil && o.TypeHints != nil {
		if v, ok := o.TypeHints[path]; ok {
			return v
		}
	}
	return fallback
}

// File is a decoded GVAS archive.
type File struct {
	Header *Header
	Root   *Properties
	// Trailer is whatever follows the root property block. Normally four zero
	// bytes; anything else means the parse did not consume the whole archive,
	// so it is preserved rather than regenerated.
	Trailer []byte
}

// Decode parses a whole GVAS archive.
func Decode(data []byte, opts *Options) (*File, error) {
	r := NewReader(data)

	h, err := ReadHeader(r)
	if err != nil {
		return nil, err
	}

	root, err := readProperties(r, "", opts)
	if err != nil {
		return nil, err
	}

	return &File{Header: h, Root: root, Trailer: r.Rest()}, nil
}

// readProperties consumes named properties until the "None" terminator.
func readProperties(r *Reader, path string, opts *Options) (*Properties, error) {
	props := NewProperties()
	for {
		name, err := r.String()
		if err != nil {
			return nil, fmt.Errorf("%s: property name: %w", path, err)
		}
		if name.Value == "None" {
			// The terminator's own encoding must be reproduced, so remember
			// whether it arrived wide.
			props.terminator = name
			return props, nil
		}

		typeName, err := r.String()
		if err != nil {
			return nil, fmt.Errorf("%s.%s: property type: %w", path, name.Value, err)
		}
		size, err := r.U64()
		if err != nil {
			return nil, fmt.Errorf("%s.%s: property size: %w", path, name.Value, err)
		}

		child := path + "." + name.Value
		v, err := readProperty(r, typeName.Value, size, child, opts)
		if err != nil {
			return nil, err
		}

		props.Names = append(props.Names, name)
		props.types = append(props.types, typeName)
		if props.Values == nil {
			props.Values = map[string]Property{}
		}
		props.Values[name.Value] = v
	}
}

func readProperty(r *Reader, typeName string, size uint64, path string, opts *Options) (Property, error) {
	switch typeName {
	case "IntProperty":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		v, err := r.I32()
		return &IntProperty{ID: id, Value: v}, wrap(path, err)

	case "UInt16Property":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		v, err := r.U16()
		return &UInt16Property{ID: id, Value: v}, wrap(path, err)

	case "UInt32Property":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		v, err := r.U32()
		return &UInt32Property{ID: id, Value: v}, wrap(path, err)

	case "UInt64Property":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		v, err := r.U64()
		return &UInt64Property{ID: id, Value: v}, wrap(path, err)

	case "Int64Property":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		v, err := r.I64()
		return &Int64Property{ID: id, Value: v}, wrap(path, err)

	case "FixedPoint64Property":
		// Stored as int32 despite the name.
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		v, err := r.I32()
		return &FixedPoint64Property{ID: id, Value: v}, wrap(path, err)

	case "FloatProperty":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		v, err := r.F32()
		return &FloatProperty{ID: id, Value: v}, wrap(path, err)

	case "DoubleProperty":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		v, err := r.F64()
		return &DoubleProperty{ID: id, Value: v}, wrap(path, err)

	case "StrProperty":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		v, err := r.String()
		return &StrProperty{ID: id, Value: v}, wrap(path, err)

	case "NameProperty":
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		v, err := r.String()
		return &NameProperty{ID: id, Value: v}, wrap(path, err)

	case "BoolProperty":
		// Value first, then the id — the reverse of every other property.
		v, err := r.Bool()
		if err != nil {
			return nil, wrap(path, err)
		}
		id, err := r.OptionalGUID()
		return &BoolProperty{ID: id, Value: v}, wrap(path, err)

	case "EnumProperty":
		enumType, err := r.String()
		if err != nil {
			return nil, wrap(path, err)
		}
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		v, err := r.String()
		return &EnumProperty{ID: id, Type: enumType, Value: v}, wrap(path, err)

	case "ByteProperty":
		enumType, err := r.String()
		if err != nil {
			return nil, wrap(path, err)
		}
		id, err := r.OptionalGUID()
		if err != nil {
			return nil, wrap(path, err)
		}
		b := &ByteProperty{ID: id, EnumType: enumType}
		if enumType.Value == "None" {
			b.Byte, err = r.U8()
		} else {
			b.Enum, err = r.String()
		}
		return b, wrap(path, err)

	case "StructProperty":
		return readStructProperty(r, path, opts)

	case "ArrayProperty":
		return readArrayProperty(r, size, path, opts)

	case "MapProperty":
		return readMapProperty(r, path, opts)

	case "SetProperty":
		return readSetProperty(r, path, opts)

	default:
		return nil, fmt.Errorf("gvas: unknown property type %q at %s", typeName, path)
	}
}

func readStructProperty(r *Reader, path string, opts *Options) (Property, error) {
	structType, err := r.String()
	if err != nil {
		return nil, wrap(path, err)
	}
	structID, err := r.GUID()
	if err != nil {
		return nil, wrap(path, err)
	}
	id, err := r.OptionalGUID()
	if err != nil {
		return nil, wrap(path, err)
	}
	val, err := readStructValue(r, structType.Value, path, opts)
	if err != nil {
		return nil, err
	}
	return &StructProperty{StructType: structType, StructID: structID, ID: id, Value: val}, nil
}

// readStructValue dispatches on the struct type. A handful have fixed layouts;
// everything else is a nested property block.
func readStructValue(r *Reader, structType, path string, opts *Options) (StructValue, error) {
	switch structType {
	case "Vector":
		x, err := r.F64()
		if err != nil {
			return nil, wrap(path, err)
		}
		y, err := r.F64()
		if err != nil {
			return nil, wrap(path, err)
		}
		z, err := r.F64()
		return &VectorValue{X: x, Y: y, Z: z}, wrap(path, err)

	case "Quat":
		x, err := r.F64()
		if err != nil {
			return nil, wrap(path, err)
		}
		y, err := r.F64()
		if err != nil {
			return nil, wrap(path, err)
		}
		z, err := r.F64()
		if err != nil {
			return nil, wrap(path, err)
		}
		w, err := r.F64()
		return &QuatValue{X: x, Y: y, Z: z, W: w}, wrap(path, err)

	case "DateTime":
		v, err := r.U64()
		dt := DateTimeValue(v)
		return &dt, wrap(path, err)

	case "Guid":
		g, err := r.GUID()
		gv := GUIDValue(g)
		return &gv, wrap(path, err)

	case "LinearColor":
		rr, err := r.F32()
		if err != nil {
			return nil, wrap(path, err)
		}
		g, err := r.F32()
		if err != nil {
			return nil, wrap(path, err)
		}
		b, err := r.F32()
		if err != nil {
			return nil, wrap(path, err)
		}
		a, err := r.F32()
		return &LinearColorValue{R: rr, G: g, B: b, A: a}, wrap(path, err)

	case "Color":
		// Wire order is B, G, R, A.
		b, err := r.U8()
		if err != nil {
			return nil, wrap(path, err)
		}
		g, err := r.U8()
		if err != nil {
			return nil, wrap(path, err)
		}
		rr, err := r.U8()
		if err != nil {
			return nil, wrap(path, err)
		}
		a, err := r.U8()
		return &ColorValue{B: b, G: g, R: rr, A: a}, wrap(path, err)

	default:
		props, err := readProperties(r, path, opts)
		if err != nil {
			return nil, err
		}
		return &StructProperties{Props: props}, nil
	}
}

func readArrayProperty(r *Reader, size uint64, path string, opts *Options) (Property, error) {
	arrayType, err := r.String()
	if err != nil {
		return nil, wrap(path, err)
	}
	id, err := r.OptionalGUID()
	if err != nil {
		return nil, wrap(path, err)
	}
	a := &ArrayProperty{ArrayType: arrayType, ID: id}

	count, err := r.U32()
	if err != nil {
		return nil, wrap(path, err)
	}

	if arrayType.Value == "StructProperty" {
		sa := &StructArray{}
		if sa.PropName, err = r.String(); err != nil {
			return nil, wrap(path, err)
		}
		if sa.PropType, err = r.String(); err != nil {
			return nil, wrap(path, err)
		}
		// A size field the format writes and readers ignore; kept so the
		// re-encode reproduces it rather than recomputing something else.
		if sa.byteSize, err = r.U64(); err != nil {
			return nil, wrap(path, err)
		}
		if sa.TypeName, err = r.String(); err != nil {
			return nil, wrap(path, err)
		}
		if sa.ID, err = r.GUID(); err != nil {
			return nil, wrap(path, err)
		}
		if sa.pad, err = r.U8(); err != nil {
			return nil, wrap(path, err)
		}

		elemPath := path + "." + sa.PropName.Value
		sa.Values = make([]StructValue, 0, count)
		for i := uint32(0); i < count; i++ {
			v, err := readStructValue(r, sa.TypeName.Value, elemPath, opts)
			if err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", elemPath, i, err)
			}
			sa.Values = append(sa.Values, v)
		}
		a.Structs = sa
		return a, nil
	}

	// Flat arrays. The payload size excludes the count that precedes it.
	payload := int(size) - 4
	switch arrayType.Value {
	case "ByteProperty":
		if payload != int(count) {
			return nil, fmt.Errorf("gvas: %s: labelled ByteProperty array not supported (payload %d, count %d)",
				path, payload, count)
		}
		b, err := r.Bytes(int(count))
		a.Values.Bytes = b
		return a, wrap(path, err)

	case "EnumProperty", "NameProperty", "StrProperty":
		a.Values.Strings = make([]String, 0, count)
		for i := uint32(0); i < count; i++ {
			s, err := r.String()
			if err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", path, i, err)
			}
			a.Values.Strings = append(a.Values.Strings, s)
		}
		return a, nil

	case "Guid":
		a.Values.GUIDs = make([]GUID, 0, count)
		for i := uint32(0); i < count; i++ {
			g, err := r.GUID()
			if err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", path, i, err)
			}
			a.Values.GUIDs = append(a.Values.GUIDs, g)
		}
		return a, nil

	default:
		return nil, fmt.Errorf("gvas: unknown array element type %q at %s", arrayType.Value, path)
	}
}

func readMapProperty(r *Reader, path string, opts *Options) (Property, error) {
	m := &MapProperty{}
	var err error

	if m.KeyType, err = r.String(); err != nil {
		return nil, wrap(path, err)
	}
	if m.ValueType, err = r.String(); err != nil {
		return nil, wrap(path, err)
	}
	if m.ID, err = r.OptionalGUID(); err != nil {
		return nil, wrap(path, err)
	}
	if m.Unknown, err = r.U32(); err != nil {
		return nil, wrap(path, err)
	}
	count, err := r.U32()
	if err != nil {
		return nil, wrap(path, err)
	}

	keyPath := path + ".Key"
	valPath := path + ".Value"
	if m.KeyType.Value == "StructProperty" {
		m.KeyStructType = Str(opts.hint(keyPath, "Guid"))
	}
	if m.ValueType.Value == "StructProperty" {
		m.ValueStructType = Str(opts.hint(valPath, "StructProperty"))
	}

	m.Entries = make([]MapEntry, 0, count)
	for i := uint32(0); i < count; i++ {
		k, err := readMapValue(r, m.KeyType.Value, m.KeyStructType.Value, keyPath, opts)
		if err != nil {
			return nil, fmt.Errorf("%s[%d].Key: %w", path, i, err)
		}
		v, err := readMapValue(r, m.ValueType.Value, m.ValueStructType.Value, valPath, opts)
		if err != nil {
			return nil, fmt.Errorf("%s[%d].Value: %w", path, i, err)
		}
		m.Entries = append(m.Entries, MapEntry{Key: k, Value: v})
	}
	return m, nil
}

// readMapValue reads a key or value. Only a subset of types may appear here,
// and unlike a top-level property they carry no id or size prefix.
func readMapValue(r *Reader, typeName, structType, path string, opts *Options) (MapValue, error) {
	var mv MapValue
	switch typeName {
	case "StructProperty":
		v, err := readStructValue(r, structType, path, opts)
		mv.Struct = v
		return mv, err
	case "EnumProperty", "NameProperty", "StrProperty":
		s, err := r.String()
		mv.Scalar.Str = s
		return mv, wrap(path, err)
	case "IntProperty":
		v, err := r.I32()
		mv.Scalar.I32 = v
		return mv, wrap(path, err)
	case "Int64Property":
		v, err := r.I64()
		mv.Scalar.I64 = v
		return mv, wrap(path, err)
	case "UInt32Property":
		v, err := r.U32()
		mv.Scalar.U32 = v
		return mv, wrap(path, err)
	case "BoolProperty":
		v, err := r.Bool()
		mv.Scalar.Bool = v
		return mv, wrap(path, err)
	default:
		return mv, fmt.Errorf("gvas: unsupported map value type %q at %s", typeName, path)
	}
}

func readSetProperty(r *Reader, path string, opts *Options) (Property, error) {
	s := &SetProperty{}
	var err error

	if s.SetType, err = r.String(); err != nil {
		return nil, wrap(path, err)
	}
	if s.ID, err = r.OptionalGUID(); err != nil {
		return nil, wrap(path, err)
	}
	if s.Unknown, err = r.U32(); err != nil {
		return nil, wrap(path, err)
	}
	count, err := r.U32()
	if err != nil {
		return nil, wrap(path, err)
	}

	if s.SetType.Value == "StructProperty" {
		elemPath := path + ".StructProperty"
		s.StructType = Str(opts.hint(elemPath, "StructProperty"))
		s.Structs = make([]StructValue, 0, count)
		for i := uint32(0); i < count; i++ {
			v, err := readStructValue(r, s.StructType.Value, elemPath, opts)
			if err != nil {
				return nil, fmt.Errorf("%s[%d]: %w", path, i, err)
			}
			s.Structs = append(s.Structs, v)
		}
		return s, nil
	}

	s.Props = make([]*Properties, 0, count)
	for i := uint32(0); i < count; i++ {
		p, err := readProperties(r, path, opts)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", path, i, err)
		}
		s.Props = append(s.Props, p)
	}
	return s, nil
}

func wrap(path string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", path, err)
}
