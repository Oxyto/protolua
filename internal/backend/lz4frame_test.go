package backend

import (
	"bytes"
	"testing"
)

func TestLZ4FrameUncompressedRoundTrip(t *testing.T) {
	payload := bytes.Repeat([]byte("ProtoLua"), 1024)
	var buf bytes.Buffer
	if err := writeLZ4Frame(&buf, payload); err != nil {
		t.Fatal(err)
	}
	got, err := readLZ4Frame(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("roundtrip mismatch: got %d bytes, want %d", len(got), len(payload))
	}
}

func TestDecodeLZ4CompressedBlock(t *testing.T) {
	block := []byte{0x35, 'a', 'b', 'c', 0x03, 0x00}
	got, err := decodeLZ4Block(block)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("abcabcabcabc")
	if !bytes.Equal(got, want) {
		t.Fatalf("decoded %q, want %q", got, want)
	}
}
