package gvas

import "fmt"

// Encode serialises a decoded archive.
//
// The contract this package exists to uphold: for any archive the decoder
// accepts, Encode(Decode(b)) equals b byte for byte.
func Encode(f *File) ([]byte, error) {
	w := NewWriter()
	f.Header.Write(w)
	if err := writeProperties(w, f.Root); err != nil {
		return nil, err
	}
	w.Raw(f.Trailer)
	return w.Bytes(), nil
}

func writeProperties(w *Writer, p *Properties) error {
	for i, name := range p.Names {
		v, ok := p.Values[name.Value]
		if !ok {
			return fmt.Errorf("gvas: property %q listed but missing", name.Value)
		}

		w.String(name)
		w.String(p.TypeNameAt(i))

		// The size field does not measure the whole payload: it starts after
		// the property's own header, which includes the optional GUID. An
		// IntProperty with a GUID byte and an int32 reports 4, not 5, and a
		// BoolProperty reports 0 despite writing two bytes. So the writer for
		// each type reports where counting begins.
		size := w.ReserveU64()
		sizeStart, err := writeProperty(w, v)
		if err != nil {
			return fmt.Errorf("%s: %w", name.Value, err)
		}
		size.SetU64(uint64(w.Len() - sizeStart))
	}

	// Close the block with the same "None" the decoder saw. A block built in
	// code has no recorded terminator, so fall back to the narrow encoding.
	term := p.terminator
	if term.Value == "" {
		term = Str("None")
	}
	w.String(term)
	return nil
}

// writeProperty emits a property and returns the offset from which its size
// field is measured — always the point just past the type-specific header.
func writeProperty(w *Writer, v Property) (int, error) {
	switch p := v.(type) {
	case *IntProperty:
		w.OptionalGUID(p.ID)
		at := w.Len()
		w.I32(p.Value)
		return at, nil

	case *UInt16Property:
		w.OptionalGUID(p.ID)
		at := w.Len()
		w.U16(p.Value)
		return at, nil

	case *UInt32Property:
		w.OptionalGUID(p.ID)
		at := w.Len()
		w.U32(p.Value)
		return at, nil

	case *UInt64Property:
		w.OptionalGUID(p.ID)
		at := w.Len()
		w.U64(p.Value)
		return at, nil

	case *Int64Property:
		w.OptionalGUID(p.ID)
		at := w.Len()
		w.I64(p.Value)
		return at, nil

	case *FixedPoint64Property:
		w.OptionalGUID(p.ID)
		at := w.Len()
		w.I32(p.Value)
		return at, nil

	case *FloatProperty:
		w.OptionalGUID(p.ID)
		at := w.Len()
		w.F32(p.Value)
		return at, nil

	case *DoubleProperty:
		w.OptionalGUID(p.ID)
		at := w.Len()
		w.F64(p.Value)
		return at, nil

	case *StrProperty:
		w.OptionalGUID(p.ID)
		at := w.Len()
		w.String(p.Value)
		return at, nil

	case *NameProperty:
		w.OptionalGUID(p.ID)
		at := w.Len()
		w.String(p.Value)
		return at, nil

	case *BoolProperty:
		// Value precedes the id, and neither counts toward the size, which is
		// why a BoolProperty always reports 0.
		w.Bool(p.Value)
		w.OptionalGUID(p.ID)
		return w.Len(), nil

	case *EnumProperty:
		w.String(p.Type)
		w.OptionalGUID(p.ID)
		at := w.Len()
		w.String(p.Value)
		return at, nil

	case *ByteProperty:
		w.String(p.EnumType)
		w.OptionalGUID(p.ID)
		at := w.Len()
		if p.IsRawByte() {
			w.U8(p.Byte)
		} else {
			w.String(p.Enum)
		}
		return at, nil

	case *StructProperty:
		w.String(p.StructType)
		w.GUID(p.StructID)
		w.OptionalGUID(p.ID)
		at := w.Len()
		return at, writeStructValue(w, p.Value)

	case *ArrayProperty:
		return writeArrayProperty(w, p)

	case *MapProperty:
		return writeMapProperty(w, p)

	case *SetProperty:
		return writeSetProperty(w, p)

	default:
		return 0, fmt.Errorf("gvas: cannot encode %T", v)
	}
}

func writeStructValue(w *Writer, v StructValue) error {
	switch s := v.(type) {
	case *StructProperties:
		return writeProperties(w, s.Props)
	case *VectorValue:
		w.F64(s.X)
		w.F64(s.Y)
		w.F64(s.Z)
		return nil
	case *QuatValue:
		w.F64(s.X)
		w.F64(s.Y)
		w.F64(s.Z)
		w.F64(s.W)
		return nil
	case *DateTimeValue:
		w.U64(uint64(*s))
		return nil
	case *GUIDValue:
		w.GUID(GUID(*s))
		return nil
	case *LinearColorValue:
		w.F32(s.R)
		w.F32(s.G)
		w.F32(s.B)
		w.F32(s.A)
		return nil
	case *ColorValue:
		// Wire order is B, G, R, A.
		w.U8(s.B)
		w.U8(s.G)
		w.U8(s.R)
		w.U8(s.A)
		return nil
	default:
		return fmt.Errorf("gvas: cannot encode struct value %T", v)
	}
}

func writeArrayProperty(w *Writer, a *ArrayProperty) (int, error) {
	w.String(a.ArrayType)
	w.OptionalGUID(a.ID)
	at := w.Len()

	if a.Structs != nil {
		sa := a.Structs
		w.U32(uint32(len(sa.Values)))
		w.String(sa.PropName)
		w.String(sa.PropType)
		w.U64(sa.byteSize)
		w.String(sa.TypeName)
		w.GUID(sa.ID)
		w.U8(sa.pad)
		for i, v := range sa.Values {
			if err := writeStructValue(w, v); err != nil {
				return at, fmt.Errorf("[%d]: %w", i, err)
			}
		}
		return at, nil
	}

	switch {
	case a.Values.Bytes != nil:
		w.U32(uint32(len(a.Values.Bytes)))
		w.Raw(a.Values.Bytes)
	case a.Values.Strings != nil:
		w.U32(uint32(len(a.Values.Strings)))
		for _, s := range a.Values.Strings {
			w.String(s)
		}
	case a.Values.GUIDs != nil:
		w.U32(uint32(len(a.Values.GUIDs)))
		for _, g := range a.Values.GUIDs {
			w.GUID(g)
		}
	default:
		// An empty array still writes its zero count.
		w.U32(0)
	}
	return at, nil
}

func writeMapProperty(w *Writer, m *MapProperty) (int, error) {
	w.String(m.KeyType)
	w.String(m.ValueType)
	w.OptionalGUID(m.ID)
	at := w.Len()
	w.U32(m.Unknown)
	w.U32(uint32(len(m.Entries)))

	for i, e := range m.Entries {
		if err := writeMapValue(w, m.KeyType.Value, e.Key); err != nil {
			return at, fmt.Errorf("[%d].Key: %w", i, err)
		}
		if err := writeMapValue(w, m.ValueType.Value, e.Value); err != nil {
			return at, fmt.Errorf("[%d].Value: %w", i, err)
		}
	}
	return at, nil
}

func writeMapValue(w *Writer, typeName string, mv MapValue) error {
	switch typeName {
	case "StructProperty":
		return writeStructValue(w, mv.Struct)
	case "EnumProperty", "NameProperty", "StrProperty":
		w.String(mv.Scalar.Str)
		return nil
	case "IntProperty":
		w.I32(mv.Scalar.I32)
		return nil
	case "Int64Property":
		w.I64(mv.Scalar.I64)
		return nil
	case "UInt32Property":
		w.U32(mv.Scalar.U32)
		return nil
	case "BoolProperty":
		w.Bool(mv.Scalar.Bool)
		return nil
	default:
		return fmt.Errorf("gvas: cannot encode map value of type %q", typeName)
	}
}

func writeSetProperty(w *Writer, s *SetProperty) (int, error) {
	w.String(s.SetType)
	w.OptionalGUID(s.ID)
	at := w.Len()
	w.U32(s.Unknown)

	if s.Structs != nil {
		w.U32(uint32(len(s.Structs)))
		for i, v := range s.Structs {
			if err := writeStructValue(w, v); err != nil {
				return at, fmt.Errorf("[%d]: %w", i, err)
			}
		}
		return at, nil
	}

	w.U32(uint32(len(s.Props)))
	for i, p := range s.Props {
		if err := writeProperties(w, p); err != nil {
			return at, fmt.Errorf("[%d]: %w", i, err)
		}
	}
	return at, nil
}
