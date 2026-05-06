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

func TestAnalyzeLuaCompatibleProtoFluxSurface(t *testing.T) {
	diagnostics := analyzeSource(t, `
events.start = function()
  local root = root()
  local ui = root:find("UI")
  local text = ui:child("Label"):component("FrooxEngine.UIX.Text")
  write(text.Content, "Ready")
  drive(text.Color, color(0.2, 0.8, 1.0, 1.0))
  dyn("ProtoLua.Status"):write("Ready")
  repeat
    debug_log("tick")
    continue
  until true
end
`)
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestAnalyzeSimpleTypeInferenceAndMismatches(t *testing.T) {
	diagnostics := analyzeSource(t, `
function typed(value: float) -> (ok: bool, amount: float)
  local label: string = 12
  local inferred = true
  inferred = "no"
  output ok = value
  output amount = value + 1
end
`)
	messages := Format(diagnostics)
	for _, expected := range []string{
		`local "label" expects string, got int`,
		`assignment to "inferred" expects bool, got string`,
		`output "ok" expects bool, got float`,
	} {
		if !strings.Contains(messages, expected) {
			t.Fatalf("expected diagnostic %q in:\n%s", expected, messages)
		}
	}
}

func TestAnalyzeProtoFluxNodePortsAndStrictMode(t *testing.T) {
	diagnostics := analyzeSource(t, `
on start do
  pf.node("Actions.Write", { Wrong = 1 })
end
`)
	messages := Format(diagnostics)
	if !strings.Contains(messages, `node ProtoFlux:Write has no input port "Wrong"`) {
		t.Fatalf("expected unknown port warning, got:\n%s", messages)
	}
	if HasErrors(diagnostics) {
		t.Fatalf("unknown port should be a warning outside strict mode: %#v", diagnostics)
	}

	strict := analyzeSourceWithOptions(t, `
on start do
  pf.node("Community.Custom.Node", { Value = 1 })
  pf.unlisted_helper(1)
end
`, Options{Strict: true})
	strictMessages := Format(strict)
	for _, expected := range []string{
		`unknown ProtoFlux node "Community.Custom.Node" in strict mode`,
		`unknown ProtoFlux intrinsic "pf.unlisted_helper" in strict mode`,
	} {
		if !strings.Contains(strictMessages, expected) {
			t.Fatalf("expected strict diagnostic %q in:\n%s", expected, strictMessages)
		}
	}
	if !HasErrors(strict) {
		t.Fatalf("strict diagnostics should include errors")
	}
}

func TestAnalyzeKnownComponentFields(t *testing.T) {
	diagnostics := analyzeSource(t, `
on start do
  local root = pf.root()
  local text = pf.component(root, "FrooxEngine.UIX.Text")
  pf.debug_log(text.Content)
  pf.debug_log(text.NotAField)
end
`)
	messages := Format(diagnostics)
	if !strings.Contains(messages, `component FrooxEngine.UIX.Text has no known field "NotAField"`) {
		t.Fatalf("expected field diagnostic, got:\n%s", messages)
	}
}

func TestAnalyzeLuaStdlibSurface(t *testing.T) {
	diagnostics := analyzeSource(t, `
on start do
  local amount = math.max(1, math.abs(-2))
  local label = string.format("value: {0}", amount)
  local len = string.len(label)
  local items = {}
  table.insert(items, amount)
  require("shared")
end
`)
	if len(diagnostics) != 0 {
		t.Fatalf("expected stdlib calls to pass semantic analysis, got %#v", diagnostics)
	}

	strict := analyzeSourceWithOptions(t, `
on start do
  math.unknown(1)
end
`, Options{Strict: true})
	if !strings.Contains(Format(strict), `unknown Lua stdlib function "math.unknown" in strict mode`) {
		t.Fatalf("expected strict stdlib diagnostic, got:\n%s", Format(strict))
	}
}

func analyzeSource(t *testing.T, source string) []Diagnostic {
	return analyzeSourceWithOptions(t, source, Options{})
}

func analyzeSourceWithOptions(t *testing.T, source string, options Options) []Diagnostic {
	t.Helper()
	tokens, err := lexer.New(source).Lex()
	if err != nil {
		t.Fatal(err)
	}
	program, err := parser.Parse(tokens)
	if err != nil {
		t.Fatal(err)
	}
	return AnalyzeWithOptions(program, options)
}
