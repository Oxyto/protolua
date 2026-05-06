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

func TestLowerProtoFluxNodeResolutionMetadata(t *testing.T) {
	source := `
on start do
  node("ProtoFlux:Write", { Value = "Ready" })
  node("Community.Custom.Node", { Input = 1 })
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
	event := lowered.Nodes[0]
	known := event.Body[0]
	if known.Op != "ProtoFluxNode" {
		t.Fatalf("expected known generic ProtoFlux node, got %s", known.Op)
	}
	if known.Inputs["knownNode"] != true {
		t.Fatalf("expected ProtoFlux:Write to resolve as known, got %#v", known.Inputs)
	}
	if known.Inputs["canonicalPath"] != "ProtoFlux:Write" {
		t.Fatalf("canonicalPath = %#v, want ProtoFlux:Write", known.Inputs["canonicalPath"])
	}
	if outputs, ok := known.Inputs["nodeOutputs"].([]string); !ok || len(outputs) == 0 {
		t.Fatalf("expected known node outputs metadata, got %#v", known.Inputs["nodeOutputs"])
	}

	custom := event.Body[1]
	if custom.Op != "ProtoFluxNode" {
		t.Fatalf("expected custom generic ProtoFlux node, got %s", custom.Op)
	}
	if custom.Inputs["knownNode"] != false {
		t.Fatalf("expected custom node to remain open/unknown, got %#v", custom.Inputs)
	}
	if custom.Inputs["resolvedPath"] != "Community.Custom.Node" {
		t.Fatalf("resolvedPath = %#v, want Community.Custom.Node", custom.Inputs["resolvedPath"])
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

func TestLowerLuaCompatibleProtoFluxSurface(t *testing.T) {
	source := `
--[[
  Lua-compatible surface: event tables, prelude aliases and method calls.
]]
events.start = function()
  local root = root()
  local ui = root:find("UI")
  local text = ui:child("Label"):component("FrooxEngine.UIX.Text")
  write(text.Content, "Ready")
  drive(text.Color, color(0.2, 0.8, 1.0, 1.0))
  dyn("ProtoLua.Status"):write("Ready")
  node("Actions.Write", {
    Variable = text.Content:ref(),
    Value = "Generic",
  })
  repeat
    debug_log("tick")
    break
  until true
end

events.evaluate = function(value, threshold)
  return { passed = value >= threshold, delta = value - threshold }
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
		t.Fatalf("expected two event nodes, got %d", len(lowered.Nodes))
	}
	start := lowered.Nodes[0]
	if start.Op != "Event" || start.Inputs["name"] != "start" {
		t.Fatalf("expected start event, got %#v", start)
	}
	wantOps := []string{
		"Local",
		"Local",
		"Local",
		"ProtoFluxWrite",
		"ProtoFluxDrive",
		"WriteDynamicVariable",
		"ProtoFluxNode",
		"RepeatUntil",
	}
	if len(start.Body) != len(wantOps) {
		t.Fatalf("expected %d start body nodes, got %d", len(wantOps), len(start.Body))
	}
	for i, want := range wantOps {
		if got := start.Body[i].Op; got != want {
			t.Fatalf("start body[%d]: expected %s, got %s", i, want, got)
		}
	}
	repeat := start.Body[len(start.Body)-1]
	if repeat.Body[0].Op != "DebugLog" || repeat.Body[1].Op != "Break" {
		t.Fatalf("expected repeat body to lower debug_log/break, got %#v", repeat.Body)
	}

	evaluate := lowered.Nodes[1]
	if evaluate.Op != "Event" || evaluate.Inputs["name"] != "evaluate" {
		t.Fatalf("expected evaluate event, got %#v", evaluate)
	}
	outputs := evaluate.Inputs["outputs"].([]ast.Param)
	if len(outputs) != 2 || outputs[0].Name != "passed" || outputs[1].Name != "delta" {
		t.Fatalf("expected return table to infer named outputs, got %#v", outputs)
	}
	values := evaluate.Body[0].Inputs["values"].([]any)
	if len(values) != 2 {
		t.Fatalf("expected return table to lower to two output values, got %#v", values)
	}
}

func TestLowerLuaStdlibCallsToProtoFluxExpressions(t *testing.T) {
	source := `
events.start = function()
  local amount = math.max(1, math.abs(-2))
  local label = string.format("value", amount)
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
	start := lowered.Nodes[0]
	value := start.Body[0].Inputs["value"].(map[string]any)
	if value["op"] != "ProtoFluxStdlib" || value["path"] != "Operators.ValueMax" {
		t.Fatalf("expected math.max to lower to ProtoFluxStdlib ValueMax, got %#v", value)
	}
	label := start.Body[1].Inputs["value"].(map[string]any)
	if label["op"] != "ProtoFluxStdlib" || label["path"] != "Strings.FormatString" {
		t.Fatalf("expected string.format to lower to ProtoFluxStdlib FormatString, got %#v", label)
	}
}

func TestLowerFoldsSimpleConstantExpressions(t *testing.T) {
	source := `
events.start = function()
  local amount = 1 + 2 * 3
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
	value := lowered.Nodes[0].Body[0].Inputs["value"].(map[string]any)
	if value["op"] != "Const" || value["value"] != "7" {
		t.Fatalf("expected folded constant 7, got %#v", value)
	}
}
