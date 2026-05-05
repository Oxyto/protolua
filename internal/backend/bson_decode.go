package backend

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
)

func decodeBSONDocument(data []byte) (bsonDocument, error) {
	reader := bytes.NewReader(data)
	var length int32
	if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
		return nil, err
	}
	if length < 5 || int(length) != len(data) {
		return nil, fmt.Errorf("invalid BSON document length %d for payload length %d", length, len(data))
	}
	doc := bsonDocument{}
	for {
		typ, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if typ == 0 {
			return doc, nil
		}
		key, err := readCString(reader)
		if err != nil {
			return nil, err
		}
		value, err := decodeBSONValue(reader, typ)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		doc[key] = value
	}
}

func decodeBSONValue(reader *bytes.Reader, typ byte) (any, error) {
	switch typ {
	case 0x01:
		var bits uint64
		if err := binary.Read(reader, binary.LittleEndian, &bits); err != nil {
			return nil, err
		}
		return math.Float64frombits(bits), nil
	case 0x02:
		return readBSONString(reader)
	case 0x03:
		doc, err := readNestedDocument(reader)
		if err != nil {
			return nil, err
		}
		return doc, nil
	case 0x04:
		doc, err := readNestedDocument(reader)
		if err != nil {
			return nil, err
		}
		return documentToArray(doc), nil
	case 0x08:
		b, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		return b != 0, nil
	case 0x0A:
		return nil, nil
	case 0x10:
		var value int32
		if err := binary.Read(reader, binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		return value, nil
	case 0x12:
		var value int64
		if err := binary.Read(reader, binary.LittleEndian, &value); err != nil {
			return nil, err
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported BSON type 0x%02x", typ)
	}
}

func readNestedDocument(reader *bytes.Reader) (bsonDocument, error) {
	startLen := reader.Len()
	var length int32
	if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
		return nil, err
	}
	if length < 5 || int(length) > startLen {
		return nil, fmt.Errorf("invalid nested BSON length %d", length)
	}
	raw := make([]byte, length)
	binary.LittleEndian.PutUint32(raw[:4], uint32(length))
	if _, err := io.ReadFull(reader, raw[4:]); err != nil {
		return nil, err
	}
	return decodeBSONDocument(raw)
}

func readCString(reader *bytes.Reader) (string, error) {
	var out []byte
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		if b == 0 {
			return string(out), nil
		}
		out = append(out, b)
	}
}

func readBSONString(reader *bytes.Reader) (string, error) {
	var length int32
	if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	if length < 1 || int(length) > reader.Len() {
		return "", fmt.Errorf("invalid string length %d", length)
	}
	raw := make([]byte, length)
	if _, err := io.ReadFull(reader, raw); err != nil {
		return "", err
	}
	if raw[len(raw)-1] != 0 {
		return "", fmt.Errorf("BSON string is not null terminated")
	}
	return string(raw[:len(raw)-1]), nil
}

func documentToArray(doc bsonDocument) bsonArray {
	keys := make([]int, 0, len(doc))
	for key := range doc {
		index, err := strconv.Atoi(key)
		if err == nil {
			keys = append(keys, index)
		}
	}
	sort.Ints(keys)
	out := make(bsonArray, 0, len(keys))
	for _, index := range keys {
		out = append(out, doc[strconv.Itoa(index)])
	}
	return out
}
