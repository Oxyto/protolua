package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"protolua/internal/ast"
	"protolua/internal/ir"
)

const (
	RecordFormat = "protolua.resonite-record"
	BRSONFormat  = "protolua.experimental-brson"
)

type Options struct {
	SourcePath string
	Package    string
}

type Record struct {
	Format     string      `json:"format"`
	Version    int         `json:"version"`
	Source     string      `json:"source,omitempty"`
	Package    string      `json:"package,omitempty"`
	Backend    BackendInfo `json:"backend"`
	Root       Slot        `json:"root"`
	Graph      ProtoFlux   `json:"protoFlux"`
	Warnings   []string    `json:"warnings,omitempty"`
	OriginalIR *ir.Program `json:"originalIr,omitempty"`
}

type BackendInfo struct {
	Name         string `json:"name"`
	Target       string `json:"target"`
	Serializable bool   `json:"serializable"`
	Importable   bool   `json:"importable"`
	Notes        string `json:"notes,omitempty"`
}

type Slot struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Active     bool        `json:"active"`
	Components []Component `json:"components,omitempty"`
	Children   []Slot      `json:"children,omitempty"`
}

type Component struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Fields map[string]any `json:"fields,omitempty"`
}

type ProtoFlux struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	EntryPoints []EntryPoint `json:"entryPoints,omitempty"`
	Functions   []Function   `json:"functions,omitempty"`
	Nodes       []GraphNode  `json:"nodes,omitempty"`
	Wires       []GraphWire  `json:"wires,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

type EntryPoint struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Inputs  []Port      `json:"inputs,omitempty"`
	Outputs []Port      `json:"outputs,omitempty"`
	Ports   []GraphPort `json:"ports,omitempty"`
	Wires   []GraphWire `json:"wires,omitempty"`
	Nodes   []GraphNode `json:"nodes,omitempty"`
}

type Function struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Inputs     []Port      `json:"inputs,omitempty"`
	Outputs    []Port      `json:"outputs,omitempty"`
	ReturnType string      `json:"returnType,omitempty"`
	Ports      []GraphPort `json:"ports,omitempty"`
	Wires      []GraphWire `json:"wires,omitempty"`
	Nodes      []GraphNode `json:"nodes,omitempty"`
}

type Port struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type GraphNode struct {
	ID       string         `json:"id"`
	Op       string         `json:"op"`
	Kind     string         `json:"kind"`
	Path     string         `json:"path"`
	Inputs   map[string]any `json:"inputs,omitempty"`
	Ports    []GraphPort    `json:"ports,omitempty"`
	Wires    []GraphWire    `json:"wires,omitempty"`
	Body     []GraphNode    `json:"body,omitempty"`
	Else     []GraphNode    `json:"else,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type GraphPort struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Kind      string `json:"kind"`
	Type      string `json:"type,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
}

type GraphWire struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	From       string         `json:"from,omitempty"`
	To         string         `json:"to,omitempty"`
	SourceNode string         `json:"sourceNode,omitempty"`
	TargetNode string         `json:"targetNode,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
	NodeID   string `json:"nodeId,omitempty"`
}

type builder struct {
	next     int
	warnings []string
}

func Build(program *ast.Program, opts Options) (Record, error) {
	lowered := ir.Lower(program)
	return BuildFromIR(lowered, opts), nil
}

func BuildFromIR(lowered ir.Program, opts Options) Record {
	b := &builder{}
	name := opts.Package
	if name == "" {
		name = packageName(opts.SourcePath)
	}
	record := Record{
		Format:  RecordFormat,
		Version: 1,
		Source:  opts.SourcePath,
		Package: name,
		Backend: BackendInfo{
			Name:         "protolua-backend",
			Target:       "resonite-record-model",
			Serializable: true,
			Importable:   false,
			Notes:        "This is a deterministic ProtoLua record model. The official Resonite binary record serializer is isolated behind the brson writer.",
		},
		Root: Slot{
			ID:     b.id("slot"),
			Name:   name,
			Active: true,
			Components: []Component{
				{
					ID:   b.id("component"),
					Type: "FrooxEngine.ProtoFlux.Runtimes.Execution.ProtoFluxGraphRoot",
					Fields: map[string]any{
						"Format": "ProtoLua",
						"Source": opts.SourcePath,
					},
				},
			},
		},
		Graph: ProtoFlux{
			ID:   b.id("graph"),
			Name: name,
		},
		OriginalIR: &lowered,
	}

	for _, node := range lowered.Nodes {
		switch node.Op {
		case "Event":
			record.Graph.EntryPoints = append(record.Graph.EntryPoints, b.entryPoint(node))
		case "Function":
			record.Graph.Functions = append(record.Graph.Functions, b.function(node))
		default:
			record.Graph.Nodes = append(record.Graph.Nodes, b.graphNode(node))
		}
	}

	record.Warnings = append(record.Warnings, b.warnings...)
	record.Graph.Diagnostics = b.diagnostics(record.Warnings)
	b.connectGraph(&record)
	b.materializeGraphSlots(&record)
	return record
}

func WriteRecordJSON(w io.Writer, record Record) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(record)
}

func (b *builder) entryPoint(node ir.Node) EntryPoint {
	return EntryPoint{
		ID:      node.ID,
		Name:    stringInput(node.Inputs, "name"),
		Inputs:  portsInput(node.Inputs, "params"),
		Outputs: portsInput(node.Inputs, "outputs"),
		Nodes:   b.graphNodes(node.Body),
	}
}

func (b *builder) function(node ir.Node) Function {
	return Function{
		ID:         node.ID,
		Name:       stringInput(node.Inputs, "name"),
		Inputs:     portsInput(node.Inputs, "params"),
		Outputs:    portsInput(node.Inputs, "outputs"),
		ReturnType: stringInput(node.Inputs, "returnType"),
		Nodes:      b.graphNodes(node.Body),
	}
}

func (b *builder) graphNodes(nodes []ir.Node) []GraphNode {
	out := make([]GraphNode, 0, len(nodes))
	for _, node := range nodes {
		out = append(out, b.graphNode(node))
	}
	return out
}

func (b *builder) graphNode(node ir.Node) GraphNode {
	inputs := normalizeMap(node.Inputs)
	return GraphNode{
		ID:       node.ID,
		Op:       node.Op,
		Kind:     classify(node.Op),
		Path:     protoFluxPath(node.Op),
		Inputs:   inputs,
		Body:     b.graphNodes(node.Body),
		Else:     b.graphNodes(node.Else),
		Metadata: metadata(node.Op),
	}
}

func (b *builder) diagnostics(warnings []string) []Diagnostic {
	out := make([]Diagnostic, 0, len(warnings))
	for _, warning := range warnings {
		out = append(out, Diagnostic{Severity: "warning", Message: warning})
	}
	return out
}

func (b *builder) id(prefix string) string {
	b.next++
	return fmt.Sprintf("%s_%03d", prefix, b.next)
}

func classify(op string) string {
	switch op {
	case "If", "While", "NumericFor", "Return", "Output":
		return "control"
	case "Local", "Set":
		return "variable"
	case "ProtoFluxWrite", "ProtoFluxDrive", "ProtoFluxImpulse", "ProtoFluxNode",
		"ProtoFluxPack", "ProtoFluxUnpack", "ProtoFluxDelay", "DebugLog",
		"AddComponent", "RemoveComponent", "CreateSlot", "Destroy", "SetActive", "SetComponentEnabled",
		"ListSet", "ListAdd", "ListInsert", "ListRemove", "ListClear",
		"WriteDynamicVariable", "CreateDynamicVariable", "WriteOrCreateDynamicVariable",
		"DeleteDynamicVariable", "ClearDynamicVariables", "ClearDynamicVariablesOfType",
		"DynamicVariableSpace", "DynamicVariableDriver":
		return "action"
	case "RootSlot", "ThisSlot", "SlotRef", "FindSlot", "ChildSlot", "ParentSlot", "Children",
		"ComponentRef", "ComponentList", "GetSlot", "ComponentEnabledSource",
		"FieldSource", "FieldReference", "ReferenceToOutput", "FieldRef", "FieldListRef",
		"ListGet", "ListCount",
		"ReadDynamicVariable", "DynamicVariableInput", "DynamicVariableInputWithEvents":
		return "source"
	default:
		return "node"
	}
}

func protoFluxPath(op string) string {
	switch op {
	case "ProtoFluxWrite":
		return "Actions.Write"
	case "ProtoFluxDrive":
		return "Fields.ValueFieldDrive"
	case "ProtoFluxImpulse":
		return "Actions.Call"
	case "DebugLog":
		return "Debug.DebugLog"
	case "ReadDynamicVariable":
		return "Variables.Dynamic.ReadDynamicVariable"
	case "WriteDynamicVariable":
		return "Variables.Dynamic.WriteDynamicVariable"
	case "CreateDynamicVariable":
		return "Variables.Dynamic.CreateDynamicVariable"
	case "WriteOrCreateDynamicVariable":
		return "Variables.Dynamic.WriteOrCreateDynamicVariable"
	case "DynamicVariableInput":
		return "Variables.Dynamic.DynamicVariableInput"
	case "DynamicVariableInputWithEvents":
		return "Variables.Dynamic.DynamicVariableInputWithEvents"
	case "DynamicVariableSpace":
		return "Variables.Dynamic.DynamicVariableSpace"
	case "DynamicVariableDriver":
		return "Variables.Dynamic.DynamicVariableDriver"
	case "ProtoFluxNode":
		return "Generic.Node"
	default:
		return op
	}
}

func metadata(op string) map[string]any {
	meta := map[string]any{"sourceOp": op}
	if strings.HasPrefix(op, "ProtoFlux") {
		meta["domain"] = "ProtoFlux"
	}
	return meta
}

func normalizeMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = normalizeValue(value)
	}
	return out
}

func normalizeValue(value any) any {
	switch v := value.(type) {
	case []ir.Node:
		nodes := make([]any, 0, len(v))
		for _, node := range v {
			nodes = append(nodes, node.ID)
		}
		return nodes
	case map[string]any:
		return normalizeMap(v)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, normalizeValue(item))
		}
		return out
	default:
		return v
	}
}

func portsInput(inputs map[string]any, name string) []Port {
	value, ok := inputs[name]
	if !ok || value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var ports []Port
	if err := json.Unmarshal(raw, &ports); err != nil {
		return nil
	}
	return ports
}

func stringInput(inputs map[string]any, name string) string {
	value, ok := inputs[name]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func packageName(path string) string {
	if path == "" {
		return "ProtoLuaPackage"
	}
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	base = strings.TrimSuffix(base, ext)
	if base == "" {
		return "ProtoLuaPackage"
	}
	return base
}
