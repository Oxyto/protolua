package backend

import (
	"bytes"
	"testing"

	"protolua/internal/lexer"
	"protolua/internal/parser"
)

func TestBuildRecordAndExperimentalBRSON(t *testing.T) {
	source := `
on start do
  local root: Slot = pf.root()
  pf.debug_log("ready")
end
`
	tokens, err := lexer.New(source).Lex()
	if err != nil {
		t.Fatal(err)
	}
	program, err := parser.Parse(tokens)
	if err != nil {
		t.Fatal(err)
	}
	record, err := Build(program, Options{SourcePath: "test.plua"})
	if err != nil {
		t.Fatal(err)
	}
	if record.Format != RecordFormat {
		t.Fatalf("unexpected record format %s", record.Format)
	}
	if len(record.Graph.EntryPoints) != 1 {
		t.Fatalf("expected one entry point, got %d", len(record.Graph.EntryPoints))
	}

	var buf bytes.Buffer
	if err := WriteExperimentalBRSON(&buf, record); err != nil {
		t.Fatal(err)
	}
	got, err := InspectExperimentalBRSON(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != BRSONFormat {
		t.Fatalf("unexpected brson format %s", got.Format)
	}
}
