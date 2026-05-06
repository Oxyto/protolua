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
	if len(record.Root.Children) == 0 {
		t.Fatalf("expected backend to materialize ProtoFlux graph slots")
	}
	record.Root.Children = append(record.Root.Children, Slot{
		ID:     "slot_child",
		Name:   "Child",
		Active: true,
	})
	if record.Format != RecordFormat {
		t.Fatalf("unexpected record format %s", record.Format)
	}
	if len(record.Graph.EntryPoints) != 1 {
		t.Fatalf("expected one entry point, got %d", len(record.Graph.EntryPoints))
	}
	entry := record.Graph.EntryPoints[0]
	if len(entry.Wires) == 0 {
		t.Fatalf("expected entry point to expose data/impulse wires")
	}
	if !hasGraphWire(entry.Wires, "impulse") {
		t.Fatalf("expected entry point to expose impulse wires")
	}

	var buf bytes.Buffer
	if err := WriteBRSON(&buf, record); err != nil {
		t.Fatal(err)
	}
	if got := string(buf.Bytes()[:4]); got != brsonMagic {
		t.Fatalf("expected brson magic %q, got %q", brsonMagic, got)
	}
	got, err := InspectBRSON(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got.ArchiveType != brsonLZ4Archive {
		t.Fatalf("unexpected brson archive type %d", got.ArchiveType)
	}
	if got.Document["VersionNumber"] == nil {
		t.Fatalf("decoded brson document is missing VersionNumber")
	}
	types, ok := got.Document["Types"].(bsonArray)
	if !ok || len(types) == 0 {
		t.Fatalf("decoded brson document is missing Types")
	}
	object := got.Document["Object"].(bsonDocument)
	components := object["Components"].(bsonDocument)
	data := components["Data"].(bsonArray)
	component := data[0].(bsonDocument)
	if _, ok := component["Type"].(int32); !ok {
		t.Fatalf("component Type should be a ComponentTableIndex int32, got %T", component["Type"])
	}
	children := object["Children"].(bsonArray)
	graphSlot := children[0].(bsonDocument)
	if graphSlot["Name"] != "ProtoFluxGraph" {
		t.Fatalf("expected first child to be ProtoFluxGraph, got %v", graphSlot["Name"])
	}
	child := children[0].(bsonDocument)
	child = children[len(children)-1].(bsonDocument)
	if child["ParentReference"] != record.Root.ID {
		t.Fatalf("child ParentReference = %v, want %s", child["ParentReference"], record.Root.ID)
	}
	componentsPtr := object["Components"].(bsonDocument)
	if componentsPtr["ID"] == "" || componentsPtr["Data"] == nil {
		t.Fatalf("Components should be encoded as UniquePtr(Array(IComponent)): %#v", componentsPtr)
	}
}

func TestGraphConnectionsExposePortsAndWires(t *testing.T) {
	source := `
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
`
	tokens, err := lexer.New(source).Lex()
	if err != nil {
		t.Fatal(err)
	}
	program, err := parser.Parse(tokens)
	if err != nil {
		t.Fatal(err)
	}
	record, err := Build(program, Options{SourcePath: "io.plua"})
	if err != nil {
		t.Fatal(err)
	}

	entry := record.Graph.EntryPoints[0]
	if !hasGraphPort(entry.Ports, "port_entry_n3_input_value") {
		t.Fatalf("expected event input value port, got %#v", entry.Ports)
	}
	if !hasGraphPort(entry.Ports, "port_entry_n3_output_delta") {
		t.Fatalf("expected event output delta port, got %#v", entry.Ports)
	}
	if !hasGraphWireTo(entry.Wires, "port_entry_n3_output_passed") {
		t.Fatalf("expected output statement to wire into passed output")
	}

	function := record.Graph.Functions[0]
	if !hasGraphWireTo(function.Wires, "port_function_n7_output_min") {
		t.Fatalf("expected return value to wire into function min output")
	}
	if !hasGraphWireTo(function.Wires, "port_function_n7_output_max") {
		t.Fatalf("expected return value to wire into function max output")
	}
}

func TestGenericProtoFluxNodeTableInputsBecomePorts(t *testing.T) {
	source := `
on start do
  local root: Slot = pf.root()
  local text: Component = pf.component(root, "FrooxEngine.UIX.Text")
  pf.node("Actions.Write", {
    Variable = pf.ref(text.Content),
    Value = "Ready",
  })
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
	record, err := Build(program, Options{SourcePath: "generic.plua"})
	if err != nil {
		t.Fatal(err)
	}

	var generic *GraphNode
	for i := range record.Graph.EntryPoints[0].Nodes {
		if record.Graph.EntryPoints[0].Nodes[i].Op == "ProtoFluxNode" {
			generic = &record.Graph.EntryPoints[0].Nodes[i]
			break
		}
	}
	if generic == nil {
		t.Fatalf("expected generic ProtoFlux node")
	}
	if !hasGraphPort(generic.Ports, "port_node_n3_data_in_Variable") {
		t.Fatalf("expected generic node Variable input port, got %#v", generic.Ports)
	}
	if !hasGraphPort(generic.Ports, "port_node_n3_data_in_Value") {
		t.Fatalf("expected generic node Value input port, got %#v", generic.Ports)
	}
	if !hasGraphWire(generic.Wires, "reference") {
		t.Fatalf("expected field reference wire into generic node, got %#v", generic.Wires)
	}
}

func TestGenericProtoFluxNodeUsesResolvedCatalogPorts(t *testing.T) {
	source := `
on start do
  pf.node("ProtoFlux:Write", {
    Value = "Ready",
  })
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
	record, err := Build(program, Options{SourcePath: "catalog.plua"})
	if err != nil {
		t.Fatal(err)
	}

	generic := record.Graph.EntryPoints[0].Nodes[0]
	if generic.Path != "Actions.Write" {
		t.Fatalf("generic path = %q, want Actions.Write", generic.Path)
	}
	if generic.Metadata["knownNode"] != true {
		t.Fatalf("expected knownNode metadata, got %#v", generic.Metadata)
	}
	if !hasGraphPortName(generic.Ports, "Variable", "input") {
		t.Fatalf("expected catalogued Variable input port, got %#v", generic.Ports)
	}
	if !hasGraphPortName(generic.Ports, "Next", "output") {
		t.Fatalf("expected catalogued Next output port, got %#v", generic.Ports)
	}
}

func TestFieldAccessInfersSourceOrReferenceFromPortKind(t *testing.T) {
	source := `
on start do
  local root: Slot = pf.root()
  local text: Component = pf.component(root, "FrooxEngine.UIX.Text")
  pf.debug_log(text.Content)
  pf.node("Actions.Write", {
    Variable = text.Content,
    Value = "Ready",
  })
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
	record, err := Build(program, Options{SourcePath: "fields.plua"})
	if err != nil {
		t.Fatal(err)
	}
	entry := record.Graph.EntryPoints[0]
	var debugNode, writeNode *GraphNode
	for i := range entry.Nodes {
		switch entry.Nodes[i].Op {
		case "DebugLog":
			debugNode = &entry.Nodes[i]
		case "ProtoFluxNode":
			writeNode = &entry.Nodes[i]
		}
	}
	if debugNode == nil || writeNode == nil {
		t.Fatalf("expected debug and generic write nodes, got %#v", entry.Nodes)
	}
	if !hasGraphWire(debugNode.Wires, "data") {
		t.Fatalf("expected field read to become data/source wire, got %#v", debugNode.Wires)
	}
	if !hasGraphWire(writeNode.Wires, "reference") {
		t.Fatalf("expected Write.Variable field to become reference wire, got %#v", writeNode.Wires)
	}
}

func hasGraphPort(ports []GraphPort, id string) bool {
	for _, port := range ports {
		if port.ID == id {
			return true
		}
	}
	return false
}

func hasGraphPortName(ports []GraphPort, name, direction string) bool {
	for _, port := range ports {
		if port.Name == name && port.Direction == direction {
			return true
		}
	}
	return false
}

func hasGraphWire(wires []GraphWire, kind string) bool {
	for _, wire := range wires {
		if wire.Kind == kind {
			return true
		}
	}
	return false
}

func hasGraphWireTo(wires []GraphWire, target string) bool {
	for _, wire := range wires {
		if wire.To == target {
			return true
		}
	}
	return false
}
