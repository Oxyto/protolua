package backend

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
)

const (
	lz4FrameMagic      = 0x184D2204
	lz4BlockMax4MB     = 4 * 1024 * 1024
	lz4UncompressedBit = 0x80000000
)

func writeLZ4Frame(w io.Writer, payload []byte) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(lz4FrameMagic)); err != nil {
		return err
	}
	descriptor := []byte{0x60, 0x70}
	if _, err := w.Write(descriptor); err != nil {
		return err
	}
	if _, err := w.Write([]byte{byte((xxhash32(descriptor, 0) >> 8) & 0xff)}); err != nil {
		return err
	}
	for len(payload) > 0 {
		size := len(payload)
		if size > lz4BlockMax4MB {
			size = lz4BlockMax4MB
		}
		blockHeader := uint32(size) | lz4UncompressedBit
		if err := binary.Write(w, binary.LittleEndian, blockHeader); err != nil {
			return err
		}
		if _, err := w.Write(payload[:size]); err != nil {
			return err
		}
		payload = payload[size:]
	}
	return binary.Write(w, binary.LittleEndian, uint32(0))
}

func readLZ4Frame(data []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	var magic uint32
	if err := binary.Read(reader, binary.LittleEndian, &magic); err != nil {
		return nil, err
	}
	if magic != lz4FrameMagic {
		return nil, fmt.Errorf("invalid LZ4 frame magic 0x%08x", magic)
	}
	flg, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	bd, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if flg>>6 != 1 {
		return nil, fmt.Errorf("unsupported LZ4 frame version in FLG 0x%02x", flg)
	}
	descriptor := []byte{flg, bd}
	if flg&0x08 != 0 {
		contentSize := make([]byte, 8)
		if _, err := io.ReadFull(reader, contentSize); err != nil {
			return nil, err
		}
		descriptor = append(descriptor, contentSize...)
	}
	if flg&0x01 != 0 {
		dictID := make([]byte, 4)
		if _, err := io.ReadFull(reader, dictID); err != nil {
			return nil, err
		}
		descriptor = append(descriptor, dictID...)
	}
	checksum, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	want := byte((xxhash32(descriptor, 0) >> 8) & 0xff)
	if checksum != want {
		return nil, fmt.Errorf("invalid LZ4 header checksum 0x%02x, want 0x%02x", checksum, want)
	}

	var out bytes.Buffer
	for {
		var header uint32
		if err := binary.Read(reader, binary.LittleEndian, &header); err != nil {
			return nil, err
		}
		if header == 0 {
			break
		}
		uncompressed := header&lz4UncompressedBit != 0
		size := int(header &^ lz4UncompressedBit)
		if size < 0 || size > reader.Len() {
			return nil, fmt.Errorf("invalid LZ4 block size %d", size)
		}
		block := make([]byte, size)
		if _, err := io.ReadFull(reader, block); err != nil {
			return nil, err
		}
		if !uncompressed {
			block, err = decodeLZ4Block(block)
			if err != nil {
				return nil, err
			}
		}
		out.Write(block)
		if flg&0x10 != 0 {
			if _, err := reader.Seek(4, io.SeekCurrent); err != nil {
				return nil, err
			}
		}
	}
	if flg&0x04 != 0 {
		if _, err := reader.Seek(4, io.SeekCurrent); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

func decodeLZ4Block(block []byte) ([]byte, error) {
	var out []byte
	index := 0
	for index < len(block) {
		token := block[index]
		index++

		literalLen := int(token >> 4)
		if literalLen == 15 {
			extra, next, err := readLZ4Length(block, index)
			if err != nil {
				return nil, err
			}
			literalLen += extra
			index = next
		}
		if literalLen > len(block)-index {
			return nil, fmt.Errorf("invalid LZ4 literal length %d", literalLen)
		}
		out = append(out, block[index:index+literalLen]...)
		index += literalLen

		if index == len(block) {
			return out, nil
		}
		if len(block)-index < 2 {
			return nil, fmt.Errorf("truncated LZ4 match offset")
		}
		offset := int(binary.LittleEndian.Uint16(block[index:]))
		index += 2
		if offset == 0 || offset > len(out) {
			return nil, fmt.Errorf("invalid LZ4 match offset %d", offset)
		}

		matchLen := int(token&0x0f) + 4
		if token&0x0f == 15 {
			extra, next, err := readLZ4Length(block, index)
			if err != nil {
				return nil, err
			}
			matchLen += extra
			index = next
		}
		for i := 0; i < matchLen; i++ {
			out = append(out, out[len(out)-offset])
		}
	}
	return out, nil
}

func readLZ4Length(block []byte, index int) (int, int, error) {
	length := 0
	for {
		if index >= len(block) {
			return 0, 0, fmt.Errorf("truncated LZ4 extended length")
		}
		value := int(block[index])
		index++
		length += value
		if value != 255 {
			return length, index, nil
		}
	}
}

func xxhash32(data []byte, seed uint32) uint32 {
	const (
		prime1 uint32 = 2654435761
		prime2 uint32 = 2246822519
		prime3 uint32 = 3266489917
		prime4 uint32 = 668265263
		prime5 uint32 = 374761393
	)

	var h uint32
	index := 0
	if len(data) >= 16 {
		v1 := seed + prime1 + prime2
		v2 := seed + prime2
		v3 := seed
		v4 := seed - prime1
		for len(data)-index >= 16 {
			v1 = xxhashRound(v1, binary.LittleEndian.Uint32(data[index:]))
			index += 4
			v2 = xxhashRound(v2, binary.LittleEndian.Uint32(data[index:]))
			index += 4
			v3 = xxhashRound(v3, binary.LittleEndian.Uint32(data[index:]))
			index += 4
			v4 = xxhashRound(v4, binary.LittleEndian.Uint32(data[index:]))
			index += 4
		}
		h = bits.RotateLeft32(v1, 1) + bits.RotateLeft32(v2, 7) + bits.RotateLeft32(v3, 12) + bits.RotateLeft32(v4, 18)
	} else {
		h = seed + prime5
	}

	h += uint32(len(data))
	for len(data)-index >= 4 {
		h += binary.LittleEndian.Uint32(data[index:]) * prime3
		h = bits.RotateLeft32(h, 17) * prime4
		index += 4
	}
	for index < len(data) {
		h += uint32(data[index]) * prime5
		h = bits.RotateLeft32(h, 11) * prime1
		index++
	}
	h ^= h >> 15
	h *= prime2
	h ^= h >> 13
	h *= prime3
	h ^= h >> 16
	return h
}

func xxhashRound(acc, input uint32) uint32 {
	const prime1 uint32 = 2654435761
	const prime2 uint32 = 2246822519
	acc += input * prime2
	acc = bits.RotateLeft32(acc, 13)
	acc *= prime1
	return acc
}
