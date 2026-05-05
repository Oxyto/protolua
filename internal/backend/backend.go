package backend

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
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
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

type EntryPoint struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Inputs  []Port      `json:"inputs,omitempty"`
	Outputs []Port      `json:"outputs,omitempty"`
	Nodes   []GraphNode `json:"nodes,omitempty"`
}

type Function struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Inputs     []Port      `json:"inputs,omitempty"`
	Outputs    []Port      `json:"outputs,omitempty"`
	ReturnType string      `json:"returnType,omitempty"`
	Nodes      []GraphNode `json:"nodes,omitempty"`
}

type Port struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type GraphNode struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Path     string         `json:"path"`
	Inputs   map[string]any `json:"inputs,omitempty"`
	Body     []GraphNode    `json:"body,omitempty"`
	Else     []GraphNode    `json:"else,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
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
	return record
}

func WriteRecordJSON(w io.Writer, record Record) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(record)
}

func WriteExperimentalBRSON(w io.Writer, record Record) error {
	record.Format = BRSONFormat
	record.Backend.Target = "experimental-brson-container"
	record.Backend.Importable = false
	record.Warnings = append([]string{
		"Experimental .brson carrier: this is not guaranteed to be importable by Resonite until the official binary record layout is implemented.",
	}, record.Warnings...)

	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	var zipped bytes.Buffer
	gz := gzip.NewWriter(&zipped)
	if _, err := gz.Write(payload); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	sum := sha256.Sum256(payload)

	if _, err := w.Write([]byte("PLBRSON\x00")); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(1)); err != nil {
		return err
	}
	if _, err := w.Write(sum[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint64(len(payload))); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint64(zipped.Len())); err != nil {
		return err
	}
	_, err = w.Write(zipped.Bytes())
	return err
}

func InspectExperimentalBRSON(r io.Reader) (Record, error) {
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Record{}, err
	}
	if string(header[:]) != "PLBRSON\x00" {
		return Record{}, fmt.Errorf("invalid ProtoLua brson magic")
	}
	var version uint32
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return Record{}, err
	}
	if version != 1 {
		return Record{}, fmt.Errorf("unsupported ProtoLua brson version %d", version)
	}
	var sum [32]byte
	if _, err := io.ReadFull(r, sum[:]); err != nil {
		return Record{}, err
	}
	var rawLen, zippedLen uint64
	if err := binary.Read(r, binary.LittleEndian, &rawLen); err != nil {
		return Record{}, err
	}
	if err := binary.Read(r, binary.LittleEndian, &zippedLen); err != nil {
		return Record{}, err
	}
	zipped := make([]byte, zippedLen)
	if _, err := io.ReadFull(r, zipped); err != nil {
		return Record{}, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return Record{}, err
	}
	defer gz.Close()
	payload, err := io.ReadAll(gz)
	if err != nil {
		return Record{}, err
	}
	if uint64(len(payload)) != rawLen {
		return Record{}, fmt.Errorf("payload length mismatch")
	}
	got := sha256.Sum256(payload)
	if got != sum {
		return Record{}, fmt.Errorf("payload checksum mismatch: got %s want %s", hex.EncodeToString(got[:]), hex.EncodeToString(sum[:]))
	}
	var record Record
	if err := json.Unmarshal(payload, &record); err != nil {
		return Record{}, err
	}
	return record, nil
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
	case "ProtoFluxWrite", "ProtoFluxDrive", "ProtoFluxImpulse", "AddComponent", "RemoveComponent", "CreateSlot", "Destroy", "SetActive", "SetComponentEnabled":
		return "action"
	case "RootSlot", "ThisSlot", "SlotRef", "FindSlot", "ChildSlot", "ParentSlot", "Children", "ComponentRef", "ComponentList", "FieldSource", "FieldReference", "FieldRef", "FieldListRef":
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
