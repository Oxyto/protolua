package semantic

import (
	"strings"
	"testing"

	"protolua/internal/lexer"
	"protolua/internal/parser"
)

func TestAnalyzeValidOutputsAndProtoFluxCalls(t *testing.T) {
	diagnostics := analyzeSource(t, `
on evaluate(value: float, threshold: float) -> (passed: bool, delta: float) do
  output passed = value >= threshold
  output delta = value - threshold
end

function minmax(a: float, b: float) -> (min: float, max: float)
  if a < b then
    return a, b
  end
  return b, a
end
`)
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestAnalyzeSemanticErrors(t *testing.T) {
	diagnostics := analyzeSource(t, `
on start(value: float) -> (result: float) do
  local value = 2
  output missing = value
  pf.debug_log()
  unknown = 1
end
`)
	messages := Format(diagnostics)
	for _, expected := range []string{
		`variable "value" is already declared`,
		`output "missing" is not declared`,
		`pf.debug_log expects 1 argument(s), got 0`,
		`assignment to undeclared variable "unknown"`,
		`output "result" is declared but never assigned`,
	} {
		if !strings.Contains(messages, expected) {
			t.Fatalf("expected diagnostic %q in:\n%s", expected, messages)
		}
	}
	if !HasErrors(diagnostics) {
		t.Fatalf("expected semantic errors")
	}
}

func analyzeSource(t *testing.T, source string) []Diagnostic {
	t.Helper()
	tokens, err := lexer.New(source).Lex()
	if err != nil {
		t.Fatal(err)
	}
	program, err := parser.Parse(tokens)
	if err != nil {
		t.Fatal(err)
	}
	return Analyze(program)
}
