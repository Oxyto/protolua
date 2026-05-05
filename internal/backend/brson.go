package backend

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	brsonMagic      = "FrDT"
	brsonRawArchive = 0
	brsonLZ4Archive = 1
)

type BRSONEnvelope struct {
	ArchiveType int
	Document    bsonDocument
}

func WriteBRSON(w io.Writer, record Record) error {
	envelope := buildBRSONEnvelope(record)
	payload, err := encodeBSONDocument(envelope.Document)
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(brsonMagic)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(0)); err != nil {
		return err
	}
	if err := writeVarByte(w, envelope.ArchiveType); err != nil {
		return err
	}
	if envelope.ArchiveType == brsonLZ4Archive {
		return writeLZ4Frame(w, payload)
	}
	_, err = w.Write(payload)
	return err
}

func InspectBRSON(r io.Reader) (BRSONEnvelope, error) {
	reader := bufio.NewReader(r)
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return BRSONEnvelope{}, err
	}
	if string(header) != brsonMagic {
		return BRSONEnvelope{}, fmt.Errorf("invalid brson magic %q", string(header))
	}
	var reserved uint32
	if err := binary.Read(reader, binary.LittleEndian, &reserved); err != nil {
		return BRSONEnvelope{}, err
	}
	archive, err := readVarByte(reader)
	if err != nil {
		return BRSONEnvelope{}, err
	}
	payload, err := io.ReadAll(reader)
	if err != nil {
		return BRSONEnvelope{}, err
	}
	switch archive {
	case brsonRawArchive:
	case brsonLZ4Archive:
		payload, err = readLZ4Frame(payload)
		if err != nil {
			return BRSONEnvelope{}, err
		}
	default:
		return BRSONEnvelope{}, fmt.Errorf("unsupported brson archive type %d", archive)
	}
	doc, err := decodeBSONDocument(payload)
	if err != nil {
		return BRSONEnvelope{}, err
	}
	return BRSONEnvelope{ArchiveType: archive, Document: doc}, nil
}

func buildBRSONEnvelope(record Record) BRSONEnvelope {
	record.Format = "protolua.brson-bson"
	record.Backend.Target = "resonite-brson-lz4-bson"
	record.Backend.Importable = false
	record.Backend.Notes = "Writes the public FrDT header, LZ4 frame archive and BSON reference graph. Exact Resonite component field layout is still tracked in TODO.md."
	record.Warnings = append([]string{
		"LZ4 frame mode uses uncompressed LZ4 blocks to avoid external dependencies.",
		"ProtoFlux components are represented as a normalized graph model until exact Resonite persistent component fields are mapped.",
	}, record.Warnings...)

	types := collectTypes(record)
	typeIndex := typeIndex(types)
	return BRSONEnvelope{
		ArchiveType: brsonLZ4Archive,
		Document: bsonDocument{
			"Asset":         bsonArray{},
			"FeatureFlags":  featureFlags(),
			"Object":        slotDocument(record.Root, typeIndex, ""),
			"ProtoLua":      mustJSONDocument(record),
			"TypeVersions":  typeVersions(types),
			"Types":         stringArray(types),
			"VersionNumber": int32(1),
		},
	}
}

func featureFlags() bsonDocument {
	return bsonDocument{
		"ColorManagement": false,
		"ProtoFlux":       true,
		"ResetGUID":       false,
		"TEXTURE_QUALITY": false,
		"TypeManagement":  true,
	}
}

func slotDocument(slot Slot, types map[string]int, parentID string) bsonDocument {
	components := make(bsonArray, 0, len(slot.Components))
	for _, component := range slot.Components {
		components = append(components, componentDocument(component, types))
	}
	children := make(bsonArray, 0, len(slot.Children))
	for _, child := range slot.Children {
		children = append(children, slotDocument(child, types, slot.ID))
	}
	return bsonDocument{
		"Children":        children,
		"Components":      syncList(slot.ID+"_components", components),
		"ID":              slot.ID,
		"Name":            syncValue(slot.Name),
		"OrderOffset":     int64(0),
		"ParentReference": parentID,
		"Position":        vector3Document(0, 0, 0),
		"Rotation":        quatDocument(0, 0, 0, 1),
		"Scale":           vector3Document(1, 1, 1),
		"Tag":             syncValue(""),
	}
}

func componentDocument(component Component, types map[string]int) bsonDocument {
	canonical := canonicalTypeName(component.Type)
	index, ok := types[canonical]
	if !ok {
		index = -1
	}
	return bsonDocument{
		"Data": bsonDocument{
			"Enabled":     syncValue(true),
			"Fields":      syncValue(component.Fields),
			"persistent":  syncValue(true),
			"UpdateOrder": syncValue(int32(0)),
		},
		"ID":   component.ID,
		"Type": int32(index),
	}
}

func vector3Document(x, y, z float64) bsonDocument {
	return bsonDocument{"X": x, "Y": y, "Z": z}
}

func quatDocument(x, y, z, w float64) bsonDocument {
	return bsonDocument{"W": w, "X": x, "Y": y, "Z": z}
}

func collectTypes(record Record) []string {
	seen := map[string]bool{}
	var types []string
	var walkSlot func(slot Slot)
	walkSlot = func(slot Slot) {
		for _, component := range slot.Components {
			canonical := canonicalTypeName(component.Type)
			if !seen[canonical] {
				seen[canonical] = true
				types = append(types, canonical)
			}
		}
		for _, child := range slot.Children {
			walkSlot(child)
		}
	}
	walkSlot(record.Root)
	if !seen["[FrooxEngine]FrooxEngine.Slot"] {
		types = append([]string{"[FrooxEngine]FrooxEngine.Slot"}, types...)
	}
	return types
}

func typeIndex(types []string) map[string]int {
	out := make(map[string]int, len(types))
	for index, typ := range types {
		out[typ] = index
	}
	return out
}

func typeVersions(types []string) bsonDocument {
	out := bsonDocument{}
	for _, typ := range types {
		out[typ] = int32(0)
	}
	return out
}

func canonicalTypeName(name string) string {
	if name == "" {
		return ""
	}
	if name[0] == '[' {
		return name
	}
	switch name {
	case "Slot":
		return "[FrooxEngine]FrooxEngine.Slot"
	case "Component":
		return "[FrooxEngine]FrooxEngine.Component"
	}
	if strings.HasPrefix(name, "FrooxEngine.") {
		return "[FrooxEngine]" + name
	}
	if strings.HasPrefix(name, "Elements.") {
		return "[Elements.Core]" + name
	}
	if strings.Contains(name, ".") {
		assembly := strings.SplitN(name, ".", 2)[0]
		return "[" + assembly + "]" + name
	}
	return name
}

func stringArray(items []string) bsonArray {
	out := make(bsonArray, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func mustJSONDocument(value any) bsonDocument {
	raw, err := json.Marshal(value)
	if err != nil {
		return bsonDocument{"error": err.Error()}
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return bsonDocument{"error": err.Error()}
	}
	doc, ok := anyToBSON(decoded).(bsonDocument)
	if !ok {
		return bsonDocument{"value": fmt.Sprint(decoded)}
	}
	return doc
}

func anyToBSON(value any) any {
	switch v := value.(type) {
	case nil, bool, string:
		return v
	case int:
		return int64(v)
	case int32:
		return v
	case int64:
		return v
	case float64:
		return v
	case []any:
		out := make(bsonArray, 0, len(v))
		for _, item := range v {
			out = append(out, anyToBSON(item))
		}
		return out
	case map[string]any:
		out := bsonDocument{}
		for key, item := range v {
			out[key] = anyToBSON(item)
		}
		return out
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return string(raw)
		}
		return anyToBSON(decoded)
	}
}

func writeVarByte(w io.Writer, value int) error {
	for {
		b := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		if _, err := w.Write([]byte{b}); err != nil {
			return err
		}
		if value == 0 {
			return nil
		}
	}
}

func readVarByte(r *bufio.Reader) (int, error) {
	value := 0
	shift := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= int(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, nil
		}
		shift += 7
		if shift > 28 {
			return 0, fmt.Errorf("varbyte integer too large")
		}
	}
}
