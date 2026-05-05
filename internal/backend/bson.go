package backend

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

type bsonDocument map[string]any
type bsonArray []any

func encodeBSONDocument(doc bsonDocument) ([]byte, error) {
	var body bytes.Buffer
	keys := make([]string, 0, len(doc))
	for key := range doc {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := encodeBSONElement(&body, key, doc[key]); err != nil {
			return nil, err
		}
	}
	body.WriteByte(0)

	var out bytes.Buffer
	if err := binary.Write(&out, binary.LittleEndian, int32(body.Len()+4)); err != nil {
		return nil, err
	}
	out.Write(body.Bytes())
	return out.Bytes(), nil
}

func encodeBSONElement(out *bytes.Buffer, key string, value any) error {
	switch v := value.(type) {
	case nil:
		out.WriteByte(0x0A)
		writeCString(out, key)
	case bool:
		out.WriteByte(0x08)
		writeCString(out, key)
		if v {
			out.WriteByte(1)
		} else {
			out.WriteByte(0)
		}
	case string:
		out.WriteByte(0x02)
		writeCString(out, key)
		writeString(out, v)
	case int:
		encodeBSONInt(out, key, int64(v))
	case int32:
		out.WriteByte(0x10)
		writeCString(out, key)
		_ = binary.Write(out, binary.LittleEndian, v)
	case int64:
		encodeBSONInt(out, key, v)
	case float64:
		out.WriteByte(0x01)
		writeCString(out, key)
		_ = binary.Write(out, binary.LittleEndian, math.Float64bits(v))
	case bsonDocument:
		out.WriteByte(0x03)
		writeCString(out, key)
		doc, err := encodeBSONDocument(v)
		if err != nil {
			return err
		}
		out.Write(doc)
	case map[string]any:
		out.WriteByte(0x03)
		writeCString(out, key)
		doc, err := encodeBSONDocument(bsonDocument(v))
		if err != nil {
			return err
		}
		out.Write(doc)
	case bsonArray:
		out.WriteByte(0x04)
		writeCString(out, key)
		doc, err := encodeBSONArray(v)
		if err != nil {
			return err
		}
		out.Write(doc)
	case []any:
		out.WriteByte(0x04)
		writeCString(out, key)
		doc, err := encodeBSONArray(bsonArray(v))
		if err != nil {
			return err
		}
		out.Write(doc)
	case []string:
		items := make(bsonArray, 0, len(v))
		for _, item := range v {
			items = append(items, item)
		}
		out.WriteByte(0x04)
		writeCString(out, key)
		doc, err := encodeBSONArray(items)
		if err != nil {
			return err
		}
		out.Write(doc)
	default:
		return fmt.Errorf("unsupported BSON value for %q: %T", key, value)
	}
	return nil
}

func encodeBSONInt(out *bytes.Buffer, key string, value int64) {
	if value >= math.MinInt32 && value <= math.MaxInt32 {
		out.WriteByte(0x10)
		writeCString(out, key)
		_ = binary.Write(out, binary.LittleEndian, int32(value))
		return
	}
	out.WriteByte(0x12)
	writeCString(out, key)
	_ = binary.Write(out, binary.LittleEndian, value)
}

func encodeBSONArray(items bsonArray) ([]byte, error) {
	doc := bsonDocument{}
	for index, item := range items {
		doc[fmt.Sprint(index)] = item
	}
	return encodeBSONDocument(doc)
}

func writeCString(out *bytes.Buffer, value string) {
	out.WriteString(value)
	out.WriteByte(0)
}

func writeString(out *bytes.Buffer, value string) {
	_ = binary.Write(out, binary.LittleEndian, int32(len(value)+1))
	out.WriteString(value)
	out.WriteByte(0)
}
