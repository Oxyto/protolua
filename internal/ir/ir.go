package ir

import (
	"fmt"
	"strings"

	"protolua/internal/ast"
)

type Program struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	Nodes   []Node `json:"nodes"`
}

type Node struct {
	ID     string         `json:"id"`
	Op     string         `json:"op"`
	Inputs map[string]any `json:"inputs,omitempty"`
	Body   []Node         `json:"body,omitempty"`
	Else   []Node         `json:"else,omitempty"`
}

type Builder struct {
	next int
}

func Lower(program *ast.Program) Program {
	b := &Builder{}
	return Program{
		Format:  "protolua.protoflux-ir",
		Version: 1,
		Nodes:   b.lowerStatements(program.Statements),
	}
}

func (b *Builder) lowerStatements(stmts []ast.Stmt) []Node {
	nodes := make([]Node, 0, len(stmts))
	for _, stmt := range stmts {
		nodes = append(nodes, b.lowerStmt(stmt))
	}
	return nodes
}

func (b *Builder) lowerStmt(stmt ast.Stmt) Node {
	switch s := stmt.(type) {
	case *ast.LocalStmt:
		inputs := map[string]any{"name": s.Name, "value": b.lowerExpr(s.Value)}
		if s.Type != "" {
			inputs["type"] = s.Type
		}
		return b.node("Local", inputs, nil, nil)
	case *ast.AssignStmt:
		if ident, ok := s.Target.(*ast.Identifier); ok {
			return b.node("Set", map[string]any{"name": ident.Name, "value": b.lowerExpr(s.Value)}, nil, nil)
		}
		return b.node("ProtoFluxWrite", map[string]any{"target": b.lowerExpr(s.Target), "value": b.lowerExpr(s.Value)}, nil, nil)
	case *ast.FunctionStmt:
		inputs := map[string]any{"name": s.Name, "params": s.Params}
		if s.ReturnType != "" {
			inputs["returnType"] = s.ReturnType
		}
		if len(s.Outputs) > 0 {
			inputs["outputs"] = s.Outputs
		}
		return b.node("Function", inputs, b.lowerStatements(s.Body), nil)
	case *ast.EventStmt:
		inputs := map[string]any{"name": s.Name, "params": s.Params}
		if len(s.Outputs) > 0 {
			inputs["outputs"] = s.Outputs
		}
		return b.node("Event", inputs, b.lowerStatements(s.Body), nil)
	case *ast.IfStmt:
		var currentElse []Node
		if len(s.ElseBody) > 0 {
			currentElse = b.lowerStatements(s.ElseBody)
		}
		for i := len(s.Branches) - 1; i >= 0; i-- {
			branch := s.Branches[i]
			currentElse = []Node{b.node("If", map[string]any{"condition": b.lowerExpr(branch.Condition)}, b.lowerStatements(branch.Body), currentElse)}
		}
		return currentElse[0]
	case *ast.WhileStmt:
		return b.node("While", map[string]any{"condition": b.lowerExpr(s.Condition)}, b.lowerStatements(s.Body), nil)
	case *ast.ForStmt:
		return b.node("NumericFor", map[string]any{
			"name":  s.Name,
			"start": b.lowerExpr(s.Start),
			"end":   b.lowerExpr(s.End),
			"step":  b.lowerExpr(s.Step),
		}, b.lowerStatements(s.Body), nil)
	case *ast.ReturnStmt:
		values := make([]any, 0, len(s.Values))
		for _, value := range s.Values {
			values = append(values, b.lowerExpr(value))
		}
		inputs := map[string]any{"values": values}
		if len(values) == 1 {
			inputs["value"] = values[0]
		}
		return b.node("Return", inputs, nil, nil)
	case *ast.OutputStmt:
		return b.node("Output", map[string]any{"name": s.Name, "value": b.lowerExpr(s.Value)}, nil, nil)
	case *ast.WriteStmt:
		return b.node("ProtoFluxWrite", map[string]any{"target": b.lowerExpr(s.Target), "value": b.lowerExpr(s.Value)}, nil, nil)
	case *ast.DriveStmt:
		return b.node("ProtoFluxDrive", map[string]any{"target": b.lowerExpr(s.Target), "value": b.lowerExpr(s.Value)}, nil, nil)
	case *ast.ExprStmt:
		if call, ok := s.Value.(*ast.CallExpr); ok {
			if node, ok := b.lowerCallStmt(call); ok {
				return node
			}
		}
		return b.node("Eval", map[string]any{"value": b.lowerExpr(s.Value)}, nil, nil)
	default:
		return b.node("Unknown", map[string]any{"type": fmt.Sprintf("%T", stmt)}, nil, nil)
	}
}

func (b *Builder) lowerCallStmt(call *ast.CallExpr) (Node, bool) {
	lowered := b.lowerCallExpr(call)
	inputs, ok := lowered.(map[string]any)
	if !ok {
		return Node{}, false
	}
	op, ok := inputs["op"].(string)
	if !ok || !isActionOp(op) {
		return Node{}, false
	}
	delete(inputs, "op")
	return b.node(op, inputs, nil, nil), true
}

func (b *Builder) lowerExpr(expr ast.Expr) any {
	if expr == nil {
		return nil
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		return map[string]any{"op": "Get", "name": e.Name}
	case *ast.Literal:
		return map[string]any{"op": "Const", "kind": e.Kind, "value": e.Value}
	case *ast.TableExpr:
		fields := make([]map[string]any, 0, len(e.Fields))
		for _, field := range e.Fields {
			item := map[string]any{"value": b.lowerExpr(field.Value)}
			if field.Key != "" {
				item["key"] = field.Key
			}
			if field.KeyExpr != nil {
				item["keyExpr"] = b.lowerExpr(field.KeyExpr)
			}
			fields = append(fields, item)
		}
		return map[string]any{"op": "Table", "fields": fields}
	case *ast.UnaryExpr:
		return map[string]any{"op": unaryOp(e.Op), "right": b.lowerExpr(e.Right)}
	case *ast.BinaryExpr:
		return map[string]any{"op": binaryOp(e.Op), "left": b.lowerExpr(e.Left), "right": b.lowerExpr(e.Right)}
	case *ast.MemberExpr:
		return map[string]any{"op": "FieldAccess", "object": b.lowerExpr(e.Object), "field": e.Name, "method": e.Method}
	case *ast.IndexExpr:
		return map[string]any{"op": "Index", "object": b.lowerExpr(e.Object), "index": b.lowerExpr(e.Index)}
	case *ast.CallExpr:
		return b.lowerCallExpr(e)
	default:
		return map[string]any{"op": "UnknownExpr", "type": fmt.Sprintf("%T", expr)}
	}
}

func (b *Builder) lowerCallExpr(call *ast.CallExpr) any {
	path := strings.Join(callPath(call.Callee), ".")
	args := b.lowerArgs(call.Args)

	if strings.HasPrefix(path, "pf.") {
		return b.lowerProtoFluxCall(path, call.Args)
	}
	if isConstructor(path) {
		return map[string]any{"op": "Construct", "type": path, "args": args}
	}
	if member, ok := call.Callee.(*ast.MemberExpr); ok && member.Method {
		return map[string]any{"op": "MethodCall", "receiver": b.lowerExpr(member.Object), "method": member.Name, "args": args}
	}
	return map[string]any{"op": "Call", "callee": b.lowerExpr(call.Callee), "args": args}
}

func (b *Builder) lowerProtoFluxCall(path string, args []ast.Expr) map[string]any {
	switch path {
	case "pf.root":
		return pfCall("RootSlot", nil, args, b)
	case "pf.this":
		return pfCall("ThisSlot", nil, args, b)
	case "pf.slot":
		return pfCall("SlotRef", []string{"path"}, args, b)
	case "pf.find_slot":
		return pfCall("FindSlot", []string{"source", "path"}, args, b)
	case "pf.child":
		return pfCall("ChildSlot", []string{"source", "name"}, args, b)
	case "pf.parent":
		return pfCall("ParentSlot", []string{"source"}, args, b)
	case "pf.children":
		return pfCall("Children", []string{"source"}, args, b)
	case "pf.create_slot":
		return pfCall("CreateSlot", []string{"parent", "name", "options"}, args, b)
	case "pf.destroy":
		return pfCall("Destroy", []string{"target"}, args, b)
	case "pf.set_active":
		return pfCall("SetActive", []string{"target", "active"}, args, b)
	case "pf.component":
		return pfCall("ComponentRef", []string{"slot", "type", "options"}, args, b)
	case "pf.components":
		return pfCall("ComponentList", []string{"slot", "type", "options"}, args, b)
	case "pf.get_slot":
		return pfCall("GetSlot", []string{"component"}, args, b)
	case "pf.add_component":
		return pfCall("AddComponent", []string{"slot", "type", "options"}, args, b)
	case "pf.remove_component":
		return pfCall("RemoveComponent", []string{"component"}, args, b)
	case "pf.enabled":
		return pfCall("ComponentEnabledSource", []string{"component"}, args, b)
	case "pf.set_enabled":
		return pfCall("SetComponentEnabled", []string{"component", "enabled"}, args, b)
	case "pf.field":
		return pfCall("FieldRef", []string{"component", "field"}, args, b)
	case "pf.field_list":
		return pfCall("FieldListRef", []string{"component", "field"}, args, b)
	case "pf.get":
		return pfCall("FieldSource", []string{"target"}, args, b)
	case "pf.source":
		return pfCall("FieldSource", []string{"target", "field"}, args, b)
	case "pf.reference", "pf.ref":
		return pfCall("FieldReference", []string{"target", "field"}, args, b)
	case "pf.ref_to_output", "pf.as_variable":
		return pfCall("ReferenceToOutput", []string{"reference"}, args, b)
	case "pf.write":
		return pfCall("ProtoFluxWrite", []string{"target", "value"}, args, b)
	case "pf.drive":
		return pfCall("ProtoFluxDrive", []string{"target", "value"}, args, b)
	case "pf.list.get":
		return pfCall("ListGet", []string{"list", "index"}, args, b)
	case "pf.list.count":
		return pfCall("ListCount", []string{"list"}, args, b)
	case "pf.list.set":
		return pfCall("ListSet", []string{"list", "index", "value"}, args, b)
	case "pf.list.add":
		return pfCall("ListAdd", []string{"list", "value"}, args, b)
	case "pf.list.insert":
		return pfCall("ListInsert", []string{"list", "index", "value"}, args, b)
	case "pf.list.remove":
		return pfCall("ListRemove", []string{"list", "index"}, args, b)
	case "pf.list.clear":
		return pfCall("ListClear", []string{"list"}, args, b)
	case "pf.node":
		return pfCall("ProtoFluxNode", []string{"path", "inputs", "globals", "options"}, args, b)
	case "pf.impulse":
		return pfCall("ProtoFluxImpulse", []string{"target", "port"}, args, b)
	case "pf.pack":
		return pfCall("ProtoFluxPack", []string{"slot", "options"}, args, b)
	case "pf.unpack":
		return pfCall("ProtoFluxUnpack", []string{"slot"}, args, b)
	case "pf.delay":
		return pfCall("ProtoFluxDelay", []string{"seconds"}, args, b)
	case "pf.debug_log":
		return pfCall("DebugLog", []string{"value"}, args, b)
	case "pf.dyn.read":
		return pfCall("ReadDynamicVariable", []string{"source", "path", "type"}, args, b)
	case "pf.dyn.write":
		return pfCall("WriteDynamicVariable", []string{"target", "path", "value"}, args, b)
	case "pf.dyn.create":
		return pfCall("CreateDynamicVariable", []string{"target", "path", "initialValue", "options"}, args, b)
	case "pf.dyn.write_or_create":
		return pfCall("WriteOrCreateDynamicVariable", []string{"target", "path", "value", "options"}, args, b)
	case "pf.dyn.delete":
		return pfCall("DeleteDynamicVariable", []string{"target", "path", "type"}, args, b)
	case "pf.dyn.clear":
		return pfCall("ClearDynamicVariables", []string{"target"}, args, b)
	case "pf.dyn.clear_type":
		return pfCall("ClearDynamicVariablesOfType", []string{"target", "type"}, args, b)
	case "pf.dyn.input":
		return pfCall("DynamicVariableInput", []string{"path", "type", "options"}, args, b)
	case "pf.dyn.input_events":
		return pfCall("DynamicVariableInputWithEvents", []string{"path", "type", "options"}, args, b)
	case "pf.dyn.space":
		return pfCall("DynamicVariableSpace", []string{"slot", "name", "options"}, args, b)
	case "pf.dyn.drive":
		return pfCall("DynamicVariableDriver", []string{"target", "path", "field", "options"}, args, b)
	default:
		inputs := pfCall("ProtoFluxIntrinsic", []string{"arg0", "arg1", "arg2", "arg3"}, args, b)
		inputs["name"] = strings.TrimPrefix(path, "pf.")
		return inputs
	}
}

func (b *Builder) lowerArgs(args []ast.Expr) []any {
	lowered := make([]any, 0, len(args))
	for _, arg := range args {
		lowered = append(lowered, b.lowerExpr(arg))
	}
	return lowered
}

func pfCall(op string, names []string, args []ast.Expr, b *Builder) map[string]any {
	inputs := map[string]any{"op": op}
	for i, name := range names {
		if i < len(args) {
			inputs[name] = b.lowerExpr(args[i])
		}
	}
	if len(args) > len(names) {
		extra := make([]any, 0, len(args)-len(names))
		for _, arg := range args[len(names):] {
			extra = append(extra, b.lowerExpr(arg))
		}
		inputs["extraArgs"] = extra
	}
	return inputs
}

func callPath(expr ast.Expr) []string {
	switch e := expr.(type) {
	case *ast.Identifier:
		return []string{e.Name}
	case *ast.MemberExpr:
		base := callPath(e.Object)
		if len(base) == 0 || e.Method {
			return nil
		}
		return append(base, e.Name)
	default:
		return nil
	}
}

func (b *Builder) node(op string, inputs map[string]any, body, elseBody []Node) Node {
	b.next++
	node := Node{ID: fmt.Sprintf("n%d", b.next), Op: op, Inputs: inputs}
	if len(body) > 0 {
		node.Body = body
	}
	if len(elseBody) > 0 {
		node.Else = elseBody
	}
	return node
}

func isActionOp(op string) bool {
	switch op {
	case "AddComponent", "RemoveComponent", "CreateSlot", "Destroy", "SetActive",
		"SetComponentEnabled", "ListSet", "ListAdd", "ListInsert", "ListRemove",
		"ListClear",
		"ProtoFluxWrite", "ProtoFluxDrive", "ProtoFluxNode", "ProtoFluxImpulse",
		"ProtoFluxPack", "ProtoFluxUnpack", "ProtoFluxDelay", "DebugLog",
		"WriteDynamicVariable", "CreateDynamicVariable", "WriteOrCreateDynamicVariable",
		"DeleteDynamicVariable", "ClearDynamicVariables", "ClearDynamicVariablesOfType",
		"DynamicVariableSpace", "DynamicVariableDriver", "ProtoFluxIntrinsic":
		return true
	default:
		return false
	}
}

func isConstructor(path string) bool {
	switch path {
	case "bool", "int", "float", "double", "string", "float2", "float3", "float4",
		"int2", "int3", "int4", "color", "colorX", "quat", "rect", "type":
		return true
	default:
		return false
	}
}

func binaryOp(op string) string {
	switch op {
	case "+":
		return "Add"
	case "-":
		return "Subtract"
	case "*":
		return "Multiply"
	case "/":
		return "Divide"
	case "%":
		return "Modulo"
	case "^":
		return "Power"
	case "..":
		return "Concat"
	case "==":
		return "Equal"
	case "~=":
		return "NotEqual"
	case "<":
		return "LessThan"
	case "<=":
		return "LessThanOrEqual"
	case ">":
		return "GreaterThan"
	case ">=":
		return "GreaterThanOrEqual"
	case "and":
		return "And"
	case "or":
		return "Or"
	default:
		return op
	}
}

func unaryOp(op string) string {
	switch op {
	case "-":
		return "Negate"
	case "not":
		return "Not"
	case "#":
		return "Length"
	default:
		return op
	}
}
