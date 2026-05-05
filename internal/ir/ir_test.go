package ir

import (
	"testing"

	"protolua/internal/ast"
	"protolua/internal/lexer"
	"protolua/internal/parser"
)

func TestLowerProtoFluxComponentInteractions(t *testing.T) {
	source := `
on start do
  local root: Slot = pf.root()
  local text: Component = pf.component(root, "FrooxEngine.UIX.Text")
  write text.Content = "Ready"
  drive text.Color = color(0.2, 0.8, 1.0, 1.0)
  pf.dyn.write_or_create(root, "ProtoLua.Status", "Ready", { direct = true })
  pf.node("Actions.Write", { Variable = pf.ref(text.Content), Value = "Generic" })
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
	lowered := Lower(program)
	if len(lowered.Nodes) != 1 {
		t.Fatalf("expected one event node, got %d", len(lowered.Nodes))
	}
	event := lowered.Nodes[0]
	if event.Op != "Event" {
		t.Fatalf("expected Event, got %s", event.Op)
	}

	wantOps := []string{
		"Local",
		"Local",
		"ProtoFluxWrite",
		"ProtoFluxDrive",
		"WriteOrCreateDynamicVariable",
		"ProtoFluxNode",
	}
	if len(event.Body) != len(wantOps) {
		t.Fatalf("expected %d body nodes, got %d", len(wantOps), len(event.Body))
	}
	for i, want := range wantOps {
		if got := event.Body[i].Op; got != want {
			t.Fatalf("body[%d]: expected %s, got %s", i, want, got)
		}
	}
}

func TestLowerMultipleInputsAndOutputs(t *testing.T) {
	source := `
on evaluate(value: float, threshold: float) -> (passed: bool, delta: float) do
  output passed = value >= threshold
  output delta = value - threshold
end

function minmax(a: float, b: float) -> (min: float, max: float)
  return a, b
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
	lowered := Lower(program)
	if len(lowered.Nodes) != 2 {
		t.Fatalf("expected two top-level nodes, got %d", len(lowered.Nodes))
	}

	event := lowered.Nodes[0]
	if event.Op != "Event" {
		t.Fatalf("expected Event, got %s", event.Op)
	}
	eventParams := event.Inputs["params"].([]ast.Param)
	if len(eventParams) != 2 {
		t.Fatalf("expected 2 event inputs, got %d", len(eventParams))
	}
	eventOutputs := event.Inputs["outputs"].([]ast.Param)
	if len(eventOutputs) != 2 {
		t.Fatalf("expected 2 event outputs, got %d", len(eventOutputs))
	}
	if event.Body[0].Op != "Output" || event.Body[1].Op != "Output" {
		t.Fatalf("expected event body to write named outputs, got %s and %s", event.Body[0].Op, event.Body[1].Op)
	}

	fn := lowered.Nodes[1]
	if fn.Op != "Function" {
		t.Fatalf("expected Function, got %s", fn.Op)
	}
	fnOutputs := fn.Inputs["outputs"].([]ast.Param)
	if len(fnOutputs) != 2 {
		t.Fatalf("expected 2 function outputs, got %d", len(fnOutputs))
	}
	values := fn.Body[0].Inputs["values"].([]any)
	if len(values) != 2 {
		t.Fatalf("expected 2 returned values, got %d", len(values))
	}
}
