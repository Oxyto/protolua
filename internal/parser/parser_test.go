package parser

import (
	"testing"

	"protolua/internal/ast"
	"protolua/internal/lexer"
)

func TestParseCompleteLuaTableForms(t *testing.T) {
	source := `
local values = {
  [1] = "one";
  nested = { a = 1, [2] = true, "implicit", };
  "tail",
}
`
	tokens, err := lexer.New(source).Lex()
	if err != nil {
		t.Fatal(err)
	}
	program, err := Parse(tokens)
	if err != nil {
		t.Fatal(err)
	}
	local, ok := program.Statements[0].(*ast.LocalStmt)
	if !ok {
		t.Fatalf("expected local statement, got %T", program.Statements[0])
	}
	table, ok := local.Values[0].(*ast.TableExpr)
	if !ok {
		t.Fatalf("expected table expression, got %T", local.Values[0])
	}
	if len(table.Fields) != 3 {
		t.Fatalf("expected 3 top-level fields, got %d", len(table.Fields))
	}
	if table.Fields[0].KeyExpr == nil {
		t.Fatalf("expected numeric index key expression")
	}
	nested, ok := table.Fields[1].Value.(*ast.TableExpr)
	if !ok {
		t.Fatalf("expected nested table, got %T", table.Fields[1].Value)
	}
	if len(nested.Fields) != 3 {
		t.Fatalf("expected 3 nested fields, got %d", len(nested.Fields))
	}
	if nested.Fields[2].Key != "" || nested.Fields[2].KeyExpr != nil {
		t.Fatalf("expected implicit nested field, got %#v", nested.Fields[2])
	}
}
