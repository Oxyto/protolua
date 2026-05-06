package backend

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

type symbolBinding struct {
	PortID string
	NodeID string
	Type   string
}

type connectionScope struct {
	symbols     map[string]symbolBinding
	outputs     map[string]string
	outputOrder []string
}

func (b *builder) connectGraph(record *Record) {
	for i := range record.Graph.EntryPoints {
		entry := &record.Graph.EntryPoints[i]
		scope := newConnectionScope()
		entry.Ports = append(entry.Ports, boundaryPorts("entry", entry.ID, entry.Inputs, "input", "output")...)
		for _, input := range entry.Inputs {
			scope.symbols[input.Name] = symbolBinding{
				PortID: boundaryPortID("entry", entry.ID, "input", input.Name),
				NodeID: entry.ID,
				Type:   input.Type,
			}
		}
		entry.Ports = append(entry.Ports, boundaryPorts("entry", entry.ID, entry.Outputs, "output", "input")...)
		for _, output := range entry.Outputs {
			scope.outputs[output.Name] = boundaryPortID("entry", entry.ID, "output", output.Name)
			scope.outputOrder = append(scope.outputOrder, output.Name)
		}
		entry.Nodes, entry.Wires = b.connectBlock(entry.ID, entry.Nodes, scope)
	}

	for i := range record.Graph.Functions {
		function := &record.Graph.Functions[i]
		scope := newConnectionScope()
		function.Ports = append(function.Ports, boundaryPorts("function", function.ID, function.Inputs, "input", "output")...)
		for _, input := range function.Inputs {
			scope.symbols[input.Name] = symbolBinding{
				PortID: boundaryPortID("function", function.ID, "input", input.Name),
				NodeID: function.ID,
				Type:   input.Type,
			}
		}

		outputs := function.Outputs
		if len(outputs) == 0 && function.ReturnType != "" {
			outputs = []Port{{Name: "return", Type: function.ReturnType}}
		}
		function.Ports = append(function.Ports, boundaryPorts("function", function.ID, outputs, "output", "input")...)
		for _, output := range outputs {
			scope.outputs[output.Name] = boundaryPortID("function", function.ID, "output", output.Name)
			scope.outputOrder = append(scope.outputOrder, output.Name)
		}
		function.Nodes, function.Wires = b.connectBlock(function.ID, function.Nodes, scope)
	}

	scope := newConnectionScope()
	record.Graph.Nodes, record.Graph.Wires = b.connectBlock(record.Graph.ID, record.Graph.Nodes, scope)
}

func newConnectionScope() *connectionScope {
	return &connectionScope{
		symbols: map[string]symbolBinding{},
		outputs: map[string]string{},
	}
}

func (s *connectionScope) clone() *connectionScope {
	out := newConnectionScope()
	for name, binding := range s.symbols {
		out.symbols[name] = binding
	}
	for name, portID := range s.outputs {
		out.outputs[name] = portID
	}
	out.outputOrder = append(out.outputOrder, s.outputOrder...)
	return out
}

func boundaryPorts(ownerKind, ownerID string, ports []Port, role, direction string) []GraphPort {
	out := make([]GraphPort, 0, len(ports))
	for _, port := range ports {
		out = append(out, GraphPort{
			ID:        boundaryPortID(ownerKind, ownerID, role, port.Name),
			Name:      port.Name,
			Direction: direction,
			Kind:      "data",
			Type:      port.Type,
			Symbol:    port.Name,
		})
	}
	return out
}

func boundaryPortID(ownerKind, ownerID, role, name string) string {
	return "port_" + ownerKind + "_" + sanitizeID(ownerID) + "_" + role + "_" + sanitizeID(name)
}

func (b *builder) connectBlock(ownerID string, nodes []GraphNode, scope *connectionScope) ([]GraphNode, []GraphWire) {
	connected := make([]GraphNode, 0, len(nodes))
	wires := []GraphWire{}
	var previousOut string
	var previousNode string

	for _, node := range nodes {
		node, localWires := b.connectNode(node, scope)
		wires = append(wires, localWires...)

		currentIn := nodePortID(node.ID, "impulse", "in")
		if previousOut != "" && hasPort(node.Ports, currentIn) {
			wire := GraphWire{
				ID:         wireID("impulse", ownerID, previousOut, currentIn),
				Kind:       "impulse",
				From:       previousOut,
				To:         currentIn,
				SourceNode: previousNode,
				TargetNode: node.ID,
			}
			wires = append(wires, wire)
			node.Wires = append(node.Wires, wire)
		}

		if len(node.Body) > 0 {
			bodyScope := scope.clone()
			body, bodyWires := b.connectBlock(node.ID+"_body", node.Body, bodyScope)
			node.Body = body
			node.Wires = append(node.Wires, bodyWires...)
			wires = append(wires, bodyWires...)
		}
		if len(node.Else) > 0 {
			elseScope := scope.clone()
			elseBody, elseWires := b.connectBlock(node.ID+"_else", node.Else, elseScope)
			node.Else = elseBody
			node.Wires = append(node.Wires, elseWires...)
			wires = append(wires, elseWires...)
		}

		publishNodeOutputs(node, scope)
		connected = append(connected, node)

		currentOut := nodePortID(node.ID, "impulse", "out")
		if hasPort(node.Ports, currentOut) {
			previousOut = currentOut
			previousNode = node.ID
		}
	}

	return connected, wires
}

func (b *builder) connectNode(node GraphNode, scope *connectionScope) (GraphNode, []GraphWire) {
	node.Ports = nil
	node.Wires = nil
	wires := []GraphWire{}

	if nodeNeedsImpulse(node) {
		node.Ports = append(node.Ports,
			GraphPort{ID: nodePortID(node.ID, "impulse", "in"), Name: "In", Direction: "input", Kind: "impulse"},
			GraphPort{ID: nodePortID(node.ID, "impulse", "out"), Name: "Out", Direction: "output", Kind: "impulse"},
		)
	}

	for _, key := range sortedInputKeys(node.Inputs) {
		if !isDataInput(node, key) {
			continue
		}
		inputPort := GraphPort{
			ID:        nodePortID(node.ID, "data", "in_"+key),
			Name:      key,
			Direction: "input",
			Kind:      inputKind(node, key),
		}
		node.Ports = append(node.Ports, inputPort)
		inputWires := dataWiresForExpr(node.ID, node.Inputs[key], inputPort.ID, scope, inputPort.Kind)
		wires = append(wires, inputWires...)
	}

	if node.Op == "Return" {
		if values, ok := node.Inputs["values"].([]any); ok {
			for index, value := range values {
				name := fmt.Sprintf("value%d", index)
				port := GraphPort{
					ID:        nodePortID(node.ID, "data", "in_"+name),
					Name:      name,
					Direction: "input",
					Kind:      "data",
				}
				node.Ports = append(node.Ports, port)
				wires = append(wires, dataWiresForExpr(node.ID, value, port.ID, scope, port.Kind)...)
			}
		}
	}

	providedProtoFluxInputs := map[string]bool{}
	if table, ok := node.Inputs["inputs"].(map[string]any); ok {
		for _, field := range tableFields(table) {
			providedProtoFluxInputs[field.Name] = true
			kind := protoFluxInputKind(field.Name)
			port := GraphPort{
				ID:        nodePortID(node.ID, "data", "in_"+field.Name),
				Name:      field.Name,
				Direction: "input",
				Kind:      kind,
			}
			node.Ports = append(node.Ports, port)
			wires = append(wires, dataWiresForExpr(node.ID, field.Value, port.ID, scope, port.Kind)...)
		}
	}
	if node.Op == "ProtoFluxNode" {
		for _, name := range nodeStringSlice(node.Inputs, "nodeInputs") {
			name = protoFluxPortName(name)
			if name == "" || providedProtoFluxInputs[name] {
				continue
			}
			port := GraphPort{
				ID:        nodePortID(node.ID, "data", "in_"+name),
				Name:      name,
				Direction: "input",
				Kind:      protoFluxInputKind(name),
			}
			if hasPort(node.Ports, port.ID) {
				continue
			}
			node.Ports = append(node.Ports, port)
		}
	}

	for _, port := range outputPortsForNode(node) {
		node.Ports = append(node.Ports, port)
	}

	if node.Op == "Local" || node.Op == "Set" {
		if name, ok := node.Inputs["name"].(string); ok && name != "" {
			wires = append(wires, internalDataWire(node.ID, nodePortID(node.ID, "data", "in_value"), nodePortID(node.ID, "data", "out_"+name), map[string]any{"symbol": name}))
		}
	}

	if node.Op == "Output" {
		if outputName, ok := node.Inputs["name"].(string); ok {
			if outputPort, ok := scope.outputs[outputName]; ok {
				localPort := nodePortID(node.ID, "data", "out_"+outputName)
				node.Ports = append(node.Ports, GraphPort{
					ID:        localPort,
					Name:      outputName,
					Direction: "output",
					Kind:      "data",
					Symbol:    outputName,
				})
				wires = append(wires, internalDataWire(node.ID, nodePortID(node.ID, "data", "in_value"), localPort, map[string]any{"output": outputName}))
				wires = append(wires, GraphWire{
					ID:         wireID("data", node.ID, localPort, outputPort),
					Kind:       "data",
					From:       localPort,
					To:         outputPort,
					SourceNode: node.ID,
					Metadata:   map[string]any{"output": outputName},
				})
			}
		}
	}

	if node.Op == "Return" {
		values, _ := node.Inputs["values"].([]any)
		for index := range values {
			if index >= len(scope.outputOrder) {
				continue
			}
			outputName := scope.outputOrder[index]
			if outputPort, ok := scope.outputs[outputName]; ok {
				localPort := nodePortID(node.ID, "data", "out_"+outputName)
				node.Ports = append(node.Ports, GraphPort{
					ID:        localPort,
					Name:      outputName,
					Direction: "output",
					Kind:      "data",
					Symbol:    outputName,
				})
				inputPort := nodePortID(node.ID, "data", fmt.Sprintf("in_value%d", index))
				wires = append(wires, internalDataWire(node.ID, inputPort, localPort, map[string]any{"output": outputName}))
				wires = append(wires, GraphWire{
					ID:         wireID("data", node.ID, localPort, outputPort),
					Kind:       "data",
					From:       localPort,
					To:         outputPort,
					SourceNode: node.ID,
					Metadata:   map[string]any{"output": outputName},
				})
			}
		}
	}

	node.Wires = append(node.Wires, wires...)
	return node, wires
}

func nodeNeedsImpulse(node GraphNode) bool {
	switch node.Kind {
	case "action", "control", "variable":
		return true
	default:
		return false
	}
}

func isDataInput(node GraphNode, key string) bool {
	switch key {
	case "name", "type", "returnType", "outputs", "params":
		return false
	}
	if node.Op == "Return" && (key == "value" || key == "values") {
		return false
	}
	if node.Op == "ProtoFluxNode" {
		switch key {
		case "inputs", "path", "globals", "options", "resolvedPath", "canonicalPath",
			"nodeName", "nodeCategory", "knownNode", "nodeInputs", "nodeOutputs":
			return false
		}
	}
	return true
}

func inputKind(node GraphNode, key string) string {
	switch {
	case key == "target" || key == "reference" || strings.Contains(strings.ToLower(key), "ref"):
		return "reference"
	case node.Op == "ProtoFluxDrive" && key == "value":
		return "drive"
	default:
		return "data"
	}
}

func protoFluxInputKind(name string) string {
	switch strings.ToLower(name) {
	case "variable", "reference", "field":
		return "reference"
	default:
		if strings.Contains(strings.ToLower(name), "ref") {
			return "reference"
		}
		return "data"
	}
}

func outputPortsForNode(node GraphNode) []GraphPort {
	switch node.Op {
	case "Local", "Set":
		name, ok := node.Inputs["name"].(string)
		if !ok || name == "" {
			return nil
		}
		typ, _ := node.Inputs["type"].(string)
		return []GraphPort{{
			ID:        nodePortID(node.ID, "data", "out_"+name),
			Name:      name,
			Direction: "output",
			Kind:      "data",
			Type:      typ,
			Symbol:    name,
		}}
	case "ProtoFluxNode":
		return protoFluxNodeOutputPorts(node)
	default:
		if node.Kind == "source" {
			return []GraphPort{{
				ID:        nodePortID(node.ID, "data", "out_value"),
				Name:      "value",
				Direction: "output",
				Kind:      "data",
			}}
		}
	}
	return nil
}

func protoFluxNodeOutputPorts(node GraphNode) []GraphPort {
	outputs := nodeStringSlice(node.Inputs, "nodeOutputs")
	if len(outputs) == 0 {
		outputs = []string{"value"}
	}
	ports := make([]GraphPort, 0, len(outputs))
	for _, output := range outputs {
		name := protoFluxPortName(output)
		if name == "" {
			continue
		}
		kind := "data"
		if isImpulsePortName(output) {
			kind = "impulse"
		}
		ports = append(ports, GraphPort{
			ID:        nodePortID(node.ID, kind, "out_"+name),
			Name:      name,
			Direction: "output",
			Kind:      kind,
		})
	}
	if len(ports) == 0 {
		return []GraphPort{{
			ID:        nodePortID(node.ID, "data", "out_value"),
			Name:      "value",
			Direction: "output",
			Kind:      "data",
		}}
	}
	return ports
}

func nodeStringSlice(inputs map[string]any, key string) []string {
	value, ok := inputs[key]
	if !ok || value == nil {
		return nil
	}
	switch raw := value.(type) {
	case []string:
		return append([]string(nil), raw...)
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func protoFluxPortName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "*" {
		return ""
	}
	return name
}

func isImpulsePortName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "next", "true", "false", "ontrue", "onfalse", "whiletrue",
		"loopstart", "loopend", "iteration", "pressed", "pressing",
		"released", "grabbed", "touched", "touching", "selected",
		"connected", "disconnected", "onchanged":
		return true
	default:
		return false
	}
}

func internalDataWire(nodeID, from, to string, metadata map[string]any) GraphWire {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["internal"] = true
	return GraphWire{
		ID:         wireID("data", nodeID, from, to),
		Kind:       "data",
		From:       from,
		To:         to,
		SourceNode: nodeID,
		TargetNode: nodeID,
		Metadata:   metadata,
	}
}

func publishNodeOutputs(node GraphNode, scope *connectionScope) {
	for _, port := range node.Ports {
		if port.Direction != "output" || port.Symbol == "" || port.Kind != "data" {
			continue
		}
		scope.symbols[port.Symbol] = symbolBinding{
			PortID: port.ID,
			NodeID: node.ID,
			Type:   port.Type,
		}
	}
}

func dataWiresForExpr(targetNode string, expr any, targetPort string, scope *connectionScope, defaultKind ...string) []GraphWire {
	refs := expressionRefs(expr)
	wires := make([]GraphWire, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		binding, ok := scope.symbols[ref.Symbol]
		if !ok {
			continue
		}
		key := binding.PortID + ">" + targetPort + ">" + ref.Path
		if seen[key] {
			continue
		}
		seen[key] = true
		wireKind := "data"
		if ref.Kind != "" {
			wireKind = ref.Kind
		} else if len(defaultKind) > 0 && defaultKind[0] != "" && defaultKind[0] != "data" {
			wireKind = defaultKind[0]
		}
		metadata := map[string]any{"symbol": ref.Symbol}
		if ref.Path != "" {
			metadata["path"] = ref.Path
		}
		wires = append(wires, GraphWire{
			ID:         wireID(wireKind, targetNode, binding.PortID, targetPort+"_"+ref.Path),
			Kind:       wireKind,
			From:       binding.PortID,
			To:         targetPort,
			SourceNode: binding.NodeID,
			TargetNode: targetNode,
			Metadata:   metadata,
		})
	}
	return wires
}

type expressionRef struct {
	Symbol string
	Path   string
	Kind   string
}

func expressionRefs(expr any) []expressionRef {
	var refs []expressionRef
	var walk func(any, string, string)
	walk = func(value any, path, kind string) {
		switch v := value.(type) {
		case map[string]any:
			op, _ := v["op"].(string)
			switch op {
			case "Get":
				name, _ := v["name"].(string)
				if name != "" {
					refs = append(refs, expressionRef{Symbol: name, Path: strings.Trim(path, "."), Kind: kind})
				}
			case "FieldAccess":
				field, _ := v["field"].(string)
				nextPath := joinPath(path, field)
				walk(v["object"], nextPath, kind)
			case "FieldReference", "FieldRef", "ReferenceToOutput":
				walk(v["target"], path, "reference")
				walk(v["reference"], path, "reference")
			case "Const":
				return
			default:
				for key, child := range v {
					if key == "op" || key == "kind" {
						continue
					}
					walk(child, path, kind)
				}
			}
		case []any:
			for _, item := range v {
				walk(item, path, kind)
			}
		}
	}
	walk(expr, "", "")
	return refs
}

func joinPath(base, field string) string {
	if field == "" {
		return base
	}
	if base == "" {
		return field
	}
	return base + "." + field
}

type tableField struct {
	Name  string
	Value any
}

func tableFields(table map[string]any) []tableField {
	value, ok := table["fields"]
	if !ok {
		return nil
	}
	switch rawFields := value.(type) {
	case []any:
		fields := make([]tableField, 0, len(rawFields))
		for index, raw := range rawFields {
			field, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			fields = append(fields, tableFieldFromMap(index, field))
		}
		return fields
	case []map[string]any:
		fields := make([]tableField, 0, len(rawFields))
		for index, field := range rawFields {
			fields = append(fields, tableFieldFromMap(index, field))
		}
		return fields
	default:
		return nil
	}
}

func tableFieldFromMap(index int, field map[string]any) tableField {
	name := fmt.Sprintf("%d", index)
	if key, ok := field["key"].(string); ok && key != "" {
		name = key
	}
	return tableField{Name: name, Value: field["value"]}
}

func sortedInputKeys(inputs map[string]any) []string {
	keys := make([]string, 0, len(inputs))
	for key := range inputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func hasPort(ports []GraphPort, id string) bool {
	for _, port := range ports {
		if port.ID == id {
			return true
		}
	}
	return false
}

func nodePortID(nodeID, kind, name string) string {
	return "port_node_" + sanitizeID(nodeID) + "_" + kind + "_" + sanitizeID(name)
}

func wireID(kind, owner, from, to string) string {
	return "wire_" + sanitizeID(kind) + "_" + sanitizeID(owner) + "_" + sanitizeID(from) + "_to_" + sanitizeID(to)
}

func sanitizeID(value string) string {
	if value == "" {
		return "empty"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r == '_' || r == '-':
			b.WriteRune(r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "empty"
	}
	return out
}
