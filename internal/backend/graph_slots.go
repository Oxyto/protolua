package backend

import "fmt"

const (
	graphSlotComponentType      = "ProtoLua.Runtime.ProtoFluxGraph"
	entryPointSlotComponentType = "ProtoLua.Runtime.ProtoFluxEntryPoint"
	functionSlotComponentType   = "ProtoLua.Runtime.ProtoFluxFunction"
	nodeSlotComponentType       = "ProtoLua.Runtime.ProtoFluxNode"
)

func (b *builder) materializeGraphSlots(record *Record) {
	graphSlot := Slot{
		ID:     b.id("slot"),
		Name:   "ProtoFluxGraph",
		Active: true,
		Components: []Component{
			{
				ID:   b.id("component"),
				Type: graphSlotComponentType,
				Fields: map[string]any{
					"GraphID":        record.Graph.ID,
					"Name":           record.Graph.Name,
					"EntryPointRefs": b.entryPointRefs(record.Graph.EntryPoints),
					"FunctionRefs":   b.functionRefs(record.Graph.Functions),
					"Wires":          record.Graph.Wires,
				},
			},
		},
	}

	for _, entry := range record.Graph.EntryPoints {
		graphSlot.Children = append(graphSlot.Children, b.entryPointSlot(entry))
	}
	for _, function := range record.Graph.Functions {
		graphSlot.Children = append(graphSlot.Children, b.functionSlot(function))
	}
	if len(record.Graph.Nodes) > 0 {
		graphSlot.Children = append(graphSlot.Children, b.nodeGroupSlot("TopLevel", record.Graph.Nodes))
	}

	record.Root.Children = append(record.Root.Children, graphSlot)
}

func (b *builder) entryPointSlot(entry EntryPoint) Slot {
	slotID := "slot_entry_" + entry.ID
	slot := Slot{
		ID:     slotID,
		Name:   "on " + entry.Name,
		Active: true,
		Components: []Component{
			{
				ID:   "component_entry_" + entry.ID,
				Type: entryPointSlotComponentType,
				Fields: map[string]any{
					"EntryPointID": entry.ID,
					"Name":         entry.Name,
					"Inputs":       entry.Inputs,
					"Outputs":      entry.Outputs,
					"Ports":        entry.Ports,
					"Wires":        entry.Wires,
					"NodeRefs":     b.nodeRefs(entry.Nodes),
				},
			},
		},
	}
	slot.Children = b.nodeSlots(entry.Nodes)
	return slot
}

func (b *builder) functionSlot(function Function) Slot {
	slotID := "slot_function_" + function.ID
	slot := Slot{
		ID:     slotID,
		Name:   "function " + function.Name,
		Active: true,
		Components: []Component{
			{
				ID:   "component_function_" + function.ID,
				Type: functionSlotComponentType,
				Fields: map[string]any{
					"FunctionID": function.ID,
					"Name":       function.Name,
					"Inputs":     function.Inputs,
					"Outputs":    function.Outputs,
					"ReturnType": function.ReturnType,
					"Ports":      function.Ports,
					"Wires":      function.Wires,
					"NodeRefs":   b.nodeRefs(function.Nodes),
				},
			},
		},
	}
	slot.Children = b.nodeSlots(function.Nodes)
	return slot
}

func (b *builder) nodeGroupSlot(name string, nodes []GraphNode) Slot {
	slot := Slot{
		ID:     b.id("slot"),
		Name:   name,
		Active: true,
	}
	slot.Children = b.nodeSlots(nodes)
	return slot
}

func (b *builder) nodeSlots(nodes []GraphNode) []Slot {
	slots := make([]Slot, 0, len(nodes))
	for _, node := range nodes {
		slots = append(slots, b.nodeSlot(node))
	}
	return slots
}

func (b *builder) nodeSlot(node GraphNode) Slot {
	slotID := "slot_node_" + node.ID
	slot := Slot{
		ID:     slotID,
		Name:   fmt.Sprintf("%s %s", node.ID, node.Path),
		Active: true,
		Components: []Component{
			{
				ID:   "component_node_" + node.ID,
				Type: nodeSlotComponentType,
				Fields: map[string]any{
					"NodeID":   node.ID,
					"Op":       node.Op,
					"Kind":     node.Kind,
					"Path":     node.Path,
					"Inputs":   node.Inputs,
					"Ports":    node.Ports,
					"Wires":    node.Wires,
					"Metadata": node.Metadata,
					"BodyRefs": b.nodeRefs(node.Body),
					"ElseRefs": b.nodeRefs(node.Else),
				},
			},
		},
	}
	slot.Children = append(slot.Children, b.nodeGroupSlot("Body", node.Body).Children...)
	if len(node.Else) > 0 {
		slot.Children = append(slot.Children, b.nodeGroupSlot("Else", node.Else))
	}
	return slot
}

func (b *builder) entryPointRefs(entries []EntryPoint) []string {
	refs := make([]string, 0, len(entries))
	for _, entry := range entries {
		refs = append(refs, "slot_entry_"+entry.ID)
	}
	return refs
}

func (b *builder) functionRefs(functions []Function) []string {
	refs := make([]string, 0, len(functions))
	for _, function := range functions {
		refs = append(refs, "slot_function_"+function.ID)
	}
	return refs
}

func (b *builder) nodeRefs(nodes []GraphNode) []string {
	refs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		refs = append(refs, "slot_node_"+node.ID)
	}
	return refs
}
