package gvas

import "fmt"

// Property is one node of the GVAS tree.
//
// Concrete types rather than map[string]any: the shapes here are genuinely
// irregular — ByteProperty holds either a raw byte or a string depending on a
// sibling field, BoolProperty writes its value before its id while everything
// else does the reverse — and encoding those as untyped maps is how a caller
// ends up assigning an int where a nested struct belongs. That mistake writes
// a save that looks fine until the encoder fails, or worse, does not.
type Property interface {
	// TypeName is the string the archive uses to identify this property.
	TypeName() string
	isProperty()
}

// Properties is an ordered set of named properties.
//
// Order is preserved because the archive is order-sensitive: re-emitting the
// same names in a different order produces different bytes.
type Properties struct {
	Names  []String
	Values map[string]Property

	// types records the type name each property arrived under, so the encoder
	// writes back exactly what it read instead of re-deriving it.
	types []String
	// terminator is the "None" string that closed this block, kept because its
	// encoding (narrow or wide) is part of the bytes.
	terminator String
}

// TypeNameAt returns the archive type name of the i'th property.
func (p *Properties) TypeNameAt(i int) String {
	if i < len(p.types) {
		return p.types[i]
	}
	// Set() added this property after decoding, so fall back to the value's
	// own idea of its type.
	if v, ok := p.Values[p.Names[i].Value]; ok {
		return Str(v.TypeName())
	}
	return String{}
}

func NewProperties() *Properties {
	return &Properties{Values: map[string]Property{}}
}

// Get returns the property stored under name, if present.
func (p *Properties) Get(name string) (Property, bool) {
	if p == nil || p.Values == nil {
		return nil, false
	}
	v, ok := p.Values[name]
	return v, ok
}

// Has reports whether name is present.
func (p *Properties) Has(name string) bool {
	_, ok := p.Get(name)
	return ok
}

// Set replaces an existing property, keeping its position, or appends a new one.
func (p *Properties) Set(name string, v Property) {
	if p.Values == nil {
		p.Values = map[string]Property{}
	}
	if _, exists := p.Values[name]; !exists {
		p.Names = append(p.Names, Str(name))
	}
	p.Values[name] = v
}

// Delete removes a property, keeping the order of the rest.
//
// The game omits a property entirely when it holds the default, so removing
// one is a real edit rather than a cleanup — see the note about Level in
// palsave.
func (p *Properties) Delete(name string) {
	if p == nil || p.Values == nil {
		return
	}
	if _, ok := p.Values[name]; !ok {
		return
	}
	delete(p.Values, name)
	for i, n := range p.Names {
		if n.Value == name {
			p.Names = append(p.Names[:i], p.Names[i+1:]...)
			// types runs parallel to Names, so it has to lose the same slot.
			if i < len(p.types) {
				p.types = append(p.types[:i], p.types[i+1:]...)
			}
			break
		}
	}
}

// Len is the number of properties.
func (p *Properties) Len() int {
	if p == nil {
		return 0
	}
	return len(p.Names)
}

// --- leaf properties -------------------------------------------------------

// IntProperty and friends share a shape: an optional GUID then a fixed-width
// number. They stay separate types because the type name is what gets written
// back, and collapsing them would lose it.

type IntProperty struct {
	ID    *GUID
	Value int32
}

type UInt16Property struct {
	ID    *GUID
	Value uint16
}

type UInt32Property struct {
	ID    *GUID
	Value uint32
}

type UInt64Property struct {
	ID    *GUID
	Value uint64
}

type Int64Property struct {
	ID    *GUID
	Value int64
}

// FixedPoint64Property is stored as an int32 on the wire despite its name.
type FixedPoint64Property struct {
	ID    *GUID
	Value int32
}

type FloatProperty struct {
	ID    *GUID
	Value float32
}

type DoubleProperty struct {
	ID    *GUID
	Value float64
}

type StrProperty struct {
	ID    *GUID
	Value String
}

type NameProperty struct {
	ID    *GUID
	Value String
}

// BoolProperty writes its value *before* its id, unlike every other property.
type BoolProperty struct {
	ID    *GUID
	Value bool
}

// EnumProperty names both the enum and the selected member.
type EnumProperty struct {
	ID    *GUID
	Type  String
	Value String
}

// ByteProperty is the shape that most often gets mis-handled.
//
// EnumType decides what Value means: the literal string "None" means the
// payload is a single raw byte, anything else means it is an enum member name.
// Level on a pal is a ByteProperty with EnumType "None", which is why setting
// it requires reaching one level deeper than Exp does.
type ByteProperty struct {
	ID       *GUID
	EnumType String
	// Byte is meaningful when EnumType is "None".
	Byte uint8
	// Enum is meaningful when EnumType is anything else.
	Enum String
}

// IsRawByte reports whether this property carries a number rather than an
// enum member name.
func (b *ByteProperty) IsRawByte() bool { return b.EnumType.Value == "None" }

// SetByte stores a numeric value, and reports an error rather than silently
// writing a byte into an enum-typed property.
func (b *ByteProperty) SetByte(v uint8) error {
	if !b.IsRawByte() {
		return fmt.Errorf("gvas: ByteProperty holds enum %q, not a raw byte", b.EnumType.Value)
	}
	b.Byte = v
	return nil
}

func (*IntProperty) TypeName() string          { return "IntProperty" }
func (*UInt16Property) TypeName() string       { return "UInt16Property" }
func (*UInt32Property) TypeName() string       { return "UInt32Property" }
func (*UInt64Property) TypeName() string       { return "UInt64Property" }
func (*Int64Property) TypeName() string        { return "Int64Property" }
func (*FixedPoint64Property) TypeName() string { return "FixedPoint64Property" }
func (*FloatProperty) TypeName() string        { return "FloatProperty" }
func (*DoubleProperty) TypeName() string       { return "DoubleProperty" }
func (*StrProperty) TypeName() string          { return "StrProperty" }
func (*NameProperty) TypeName() string         { return "NameProperty" }
func (*BoolProperty) TypeName() string         { return "BoolProperty" }
func (*EnumProperty) TypeName() string         { return "EnumProperty" }
func (*ByteProperty) TypeName() string         { return "ByteProperty" }

func (*IntProperty) isProperty()          {}
func (*UInt16Property) isProperty()       {}
func (*UInt32Property) isProperty()       {}
func (*UInt64Property) isProperty()       {}
func (*Int64Property) isProperty()        {}
func (*FixedPoint64Property) isProperty() {}
func (*FloatProperty) isProperty()        {}
func (*DoubleProperty) isProperty()       {}
func (*StrProperty) isProperty()          {}
func (*NameProperty) isProperty()         {}
func (*BoolProperty) isProperty()         {}
func (*EnumProperty) isProperty()         {}
func (*ByteProperty) isProperty()         {}

// --- struct ---------------------------------------------------------------

// StructProperty wraps a value whose shape is chosen by StructType.
type StructProperty struct {
	StructType String
	StructID   GUID
	ID         *GUID
	Value      StructValue
}

func (*StructProperty) TypeName() string { return "StructProperty" }
func (*StructProperty) isProperty()      {}

// StructValue is the payload of a struct. A handful of struct types have fixed
// binary layouts; everything else is a nested property set.
type StructValue interface{ isStructValue() }

// StructProperties is the general case: a nested block of named properties.
type StructProperties struct {
	Props *Properties
}

// VectorValue is three doubles.
type VectorValue struct{ X, Y, Z float64 }

// QuatValue is four doubles.
type QuatValue struct{ X, Y, Z, W float64 }

// DateTimeValue is a uint64 tick count.
type DateTimeValue uint64

// GUIDValue is a bare 16-byte identifier.
type GUIDValue GUID

// LinearColorValue is four floats.
type LinearColorValue struct{ R, G, B, A float32 }

// ColorValue is four bytes, stored in BGRA order on the wire.
type ColorValue struct{ B, G, R, A uint8 }

func (*StructProperties) isStructValue() {}
func (*VectorValue) isStructValue()      {}
func (*QuatValue) isStructValue()        {}
func (*DateTimeValue) isStructValue()    {}
func (*GUIDValue) isStructValue()        {}
func (*LinearColorValue) isStructValue() {}
func (*ColorValue) isStructValue()       {}

// --- containers -----------------------------------------------------------

// ArrayProperty holds either a struct array, which carries its own inline
// header, or a flat list of scalars.
type ArrayProperty struct {
	ArrayType String
	ID        *GUID
	// Structs is set when ArrayType is "StructProperty".
	Structs *StructArray
	// Values is set otherwise.
	Values ArrayValues
	// Raw carries the untouched payload for properties handled by a custom
	// codec, so unknown Palworld blobs survive a round trip untouched.
	Raw []byte
	// CustomType records which custom codec claimed this property.
	CustomType string
}

// StructArray is an array of structs plus the inline descriptor the format
// writes once before the elements.
type StructArray struct {
	PropName String
	PropType String
	TypeName String
	ID       GUID
	Values   []StructValue

	// byteSize is a length the format records and readers ignore. Recomputing
	// it risks disagreeing with the original, so it is echoed back verbatim.
	byteSize uint64
	// pad is a single byte between the descriptor and the elements.
	pad uint8
}

// ArrayValues is a flat array payload.
type ArrayValues struct {
	// Bytes is set for ByteProperty arrays.
	Bytes []byte
	// Strings is set for Name/Enum arrays.
	Strings []String
	// GUIDs is set for Guid arrays.
	GUIDs []GUID
}

func (*ArrayProperty) TypeName() string { return "ArrayProperty" }
func (*ArrayProperty) isProperty()      {}

// MapEntry is one key/value pair of a MapProperty.
type MapEntry struct {
	Key   MapValue
	Value MapValue
}

// MapValue is a key or value inside a map or set. Only a subset of property
// types may appear here.
type MapValue struct {
	// Struct is set when the corresponding type is StructProperty.
	Struct StructValue
	// Scalar holds the value for every other permitted type.
	Scalar ScalarValue
}

// ScalarValue is a primitive appearing as a map key or value.
type ScalarValue struct {
	Str  String
	I32  int32
	I64  int64
	U32  uint32
	Bool bool
}

type MapProperty struct {
	KeyType         String
	ValueType       String
	KeyStructType   String
	ValueStructType String
	ID              *GUID
	// Unknown is the u32 the format writes before the count; observed zero but
	// preserved rather than assumed.
	Unknown uint32
	Entries []MapEntry
}

func (*MapProperty) TypeName() string { return "MapProperty" }
func (*MapProperty) isProperty()      {}

type SetProperty struct {
	SetType    String
	StructType String
	ID         *GUID
	Unknown    uint32
	// Structs is set when SetType is "StructProperty".
	Structs []StructValue
	// Props is set otherwise: each element is its own property block.
	Props []*Properties
}

func (*SetProperty) TypeName() string { return "SetProperty" }
func (*SetProperty) isProperty()      {}

// --- custom ---------------------------------------------------------------

// CustomProperty carries a property whose payload a Palworld-specific codec
// owns. Keeping the raw bytes means an unrecognised blob still round-trips.
type CustomProperty struct {
	Underlying string
	Path       string
	Raw        []byte
}

func (c *CustomProperty) TypeName() string { return c.Underlying }
func (*CustomProperty) isProperty()        {}
