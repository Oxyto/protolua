package ir

import (
	"testing"

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
