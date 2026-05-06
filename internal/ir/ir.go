package ir

import (
	"fmt"
	"strconv"
	"strings"

	"protolua/internal/ast"
	"protolua/internal/protoflux"
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

type lowerContext struct {
	outputs []ast.Param
}

func Lower(program *ast.Program) Program {
	b := &Builder{}
	return optimizeProgram(Program{
		Format:  "protolua.protoflux-ir",
		Version: 1,
		Nodes:   b.lowerStatements(program.Statements, lowerContext{}),
	})
}

func (b *Builder) lowerStatements(stmts []ast.Stmt, ctx lowerContext) []Node {
	nodes := make([]Node, 0, len(stmts))
	for _, stmt := range stmts {
		nodes = append(nodes, b.lowerStmt(stmt, ctx)...)
	}
	return nodes
}

func (b *Builder) lowerStmt(stmt ast.Stmt, ctx lowerContext) []Node {
	switch s := stmt.(type) {
	case *ast.LocalStmt:
		names := s.Names
		values := s.Values
		if len(names) == 0 {
			names = []ast.Param{{Name: s.Name, Type: s.Type}}
		}
		if len(values) == 0 && s.Value != nil {
			values = []ast.Expr{s.Value}
		}
		nodes := make([]Node, 0, len(names))
		for i, name := range names {
			var value ast.Expr
			if i < len(values) {
				value = values[i]
			}
			inputs := map[string]any{"name": name.Name, "value": b.lowerExpr(value)}
			if name.Type != "" {
				inputs["type"] = name.Type
			}
			nodes = append(nodes, b.node("Local", inputs, nil, nil))
		}
		return nodes
	case *ast.AssignStmt:
		if event, ok := b.eventFromAssignment(s); ok {
			return []Node{event}
		}
		targets := s.Targets
		values := s.Values
		if len(targets) == 0 {
			targets = []ast.Expr{s.Target}
		}
		if len(values) == 0 && s.Value != nil {
			values = []ast.Expr{s.Value}
		}
		nodes := make([]Node, 0, len(targets))
		for i, target := range targets {
			var value ast.Expr
			if i < len(values) {
				value = values[i]
			}
			if ident, ok := target.(*ast.Identifier); ok {
				nodes = append(nodes, b.node("Set", map[string]any{"name": ident.Name, "value": b.lowerExpr(value)}, nil, nil))
				continue
			}
			nodes = append(nodes, b.node("ProtoFluxWrite", map[string]any{"target": b.lowerExpr(target), "value": b.lowerExpr(value)}, nil, nil))
		}
		return nodes
	case *ast.FunctionStmt:
		if strings.HasPrefix(s.Name, "events.") {
			name := strings.TrimPrefix(s.Name, "events.")
			outputs := outputsOrInfer(s.Outputs, s.Body)
			return []Node{b.eventNode(name, s.Params, outputs, s.Body)}
		}
		inputs := map[string]any{"name": s.Name, "params": s.Params}
		if s.ReturnType != "" {
			inputs["returnType"] = s.ReturnType
		}
		if len(s.Outputs) > 0 {
			inputs["outputs"] = s.Outputs
		}
		bodyCtx := lowerContext{outputs: s.Outputs}
		return []Node{b.node("Function", inputs, b.lowerStatements(s.Body, bodyCtx), nil)}
	case *ast.LocalFunctionStmt:
		inputs := map[string]any{"name": s.Name, "params": s.Params, "local": true}
		if s.ReturnType != "" {
			inputs["returnType"] = s.ReturnType
		}
		if len(s.Outputs) > 0 {
			inputs["outputs"] = s.Outputs
		}
		bodyCtx := lowerContext{outputs: s.Outputs}
		return []Node{b.node("Function", inputs, b.lowerStatements(s.Body, bodyCtx), nil)}
	case *ast.EventStmt:
		return []Node{b.eventNode(s.Name, s.Params, s.Outputs, s.Body)}
	case *ast.IfStmt:
		var currentElse []Node
		if len(s.ElseBody) > 0 {
			currentElse = b.lowerStatements(s.ElseBody, ctx)
		}
		for i := len(s.Branches) - 1; i >= 0; i-- {
			branch := s.Branches[i]
			currentElse = []Node{b.node("If", map[string]any{"condition": b.lowerExpr(branch.Condition)}, b.lowerStatements(branch.Body, ctx), currentElse)}
		}
		return []Node{currentElse[0]}
	case *ast.WhileStmt:
		return []Node{b.node("While", map[string]any{"condition": b.lowerExpr(s.Condition)}, b.lowerStatements(s.Body, ctx), nil)}
	case *ast.RepeatStmt:
		return []Node{b.node("RepeatUntil", map[string]any{"condition": b.lowerExpr(s.Condition)}, b.lowerStatements(s.Body, ctx), nil)}
	case *ast.ForStmt:
		return []Node{b.node("NumericFor", map[string]any{
			"name":  s.Name,
			"start": b.lowerExpr(s.Start),
			"end":   b.lowerExpr(s.End),
			"step":  b.lowerExpr(s.Step),
		}, b.lowerStatements(s.Body, ctx), nil)}
	case *ast.BreakStmt:
		return []Node{b.node("Break", nil, nil, nil)}
	case *ast.ContinueStmt:
		return []Node{b.node("Continue", nil, nil, nil)}
	case *ast.ReturnStmt:
		return []Node{b.returnNode(s.Values, ctx)}
	case *ast.OutputStmt:
		return []Node{b.node("Output", map[string]any{"name": s.Name, "value": b.lowerExpr(s.Value)}, nil, nil)}
	case *ast.WriteStmt:
		return []Node{b.node("ProtoFluxWrite", map[string]any{"target": b.lowerExpr(s.Target), "value": b.lowerExpr(s.Value)}, nil, nil)}
	case *ast.DriveStmt:
		return []Node{b.node("ProtoFluxDrive", map[string]any{"target": b.lowerExpr(s.Target), "value": b.lowerExpr(s.Value)}, nil, nil)}
	case *ast.ExprStmt:
		if call, ok := s.Value.(*ast.CallExpr); ok {
			if node, ok := b.lowerCallStmt(call); ok {
				return []Node{node}
			}
		}
		return []Node{b.node("Eval", map[string]any{"value": b.lowerExpr(s.Value)}, nil, nil)}
	default:
		return []Node{b.node("Unknown", map[string]any{"type": fmt.Sprintf("%T", stmt)}, nil, nil)}
	}
}

func (b *Builder) eventNode(name string, params, outputs []ast.Param, body []ast.Stmt) Node {
	inputs := map[string]any{"name": name, "params": params}
	if len(outputs) > 0 {
		inputs["outputs"] = outputs
	}
	bodyCtx := lowerContext{outputs: outputs}
	return b.node("Event", inputs, b.lowerStatements(body, bodyCtx), nil)
}

func (b *Builder) returnNode(rawValues []ast.Expr, ctx lowerContext) Node {
	rawValues = normalizeReturnValues(rawValues, ctx.outputs)
	values := make([]any, 0, len(rawValues))
	for _, value := range rawValues {
		values = append(values, b.lowerExpr(value))
	}
	inputs := map[string]any{"values": values}
	if len(values) == 1 {
		inputs["value"] = values[0]
	}
	return b.node("Return", inputs, nil, nil)
}

func normalizeReturnValues(values []ast.Expr, outputs []ast.Param) []ast.Expr {
	if len(values) != 1 || len(outputs) == 0 {
		return values
	}
	table, ok := values[0].(*ast.TableExpr)
	if !ok {
		return values
	}
	byName := map[string]ast.Expr{}
	for _, field := range table.Fields {
		if field.Key != "" {
			byName[field.Key] = field.Value
		}
	}
	if len(byName) == 0 {
		return values
	}
	out := make([]ast.Expr, 0, len(outputs))
	for _, output := range outputs {
		value, ok := byName[output.Name]
		if !ok {
			return values
		}
		out = append(out, value)
	}
	return out
}

func outputsOrInfer(outputs []ast.Param, body []ast.Stmt) []ast.Param {
	if len(outputs) > 0 {
		return outputs
	}
	return inferOutputsFromBody(body)
}

func inferOutputsFromBody(body []ast.Stmt) []ast.Param {
	for _, stmt := range body {
		switch s := stmt.(type) {
		case *ast.ReturnStmt:
			if len(s.Values) != 1 {
				continue
			}
			table, ok := s.Values[0].(*ast.TableExpr)
			if !ok {
				continue
			}
			outputs := make([]ast.Param, 0, len(table.Fields))
			for _, field := range table.Fields {
				if field.Key != "" {
					outputs = append(outputs, ast.Param{Name: field.Key})
				}
			}
			if len(outputs) > 0 {
				return outputs
			}
		case *ast.IfStmt:
			for _, branch := range s.Branches {
				if outputs := inferOutputsFromBody(branch.Body); len(outputs) > 0 {
					return outputs
				}
			}
			if outputs := inferOutputsFromBody(s.ElseBody); len(outputs) > 0 {
				return outputs
			}
		}
	}
	return nil
}

func (b *Builder) eventFromAssignment(stmt *ast.AssignStmt) (Node, bool) {
	if len(stmt.Targets) > 1 || len(stmt.Values) != 1 {
		return Node{}, false
	}
	target, ok := stmt.Target.(*ast.MemberExpr)
	if !ok {
		return Node{}, false
	}
	if ident, ok := target.Object.(*ast.Identifier); !ok || ident.Name != "events" {
		return Node{}, false
	}
	fn, ok := stmt.Values[0].(*ast.FunctionExpr)
	if !ok {
		return Node{}, false
	}
	outputs := outputsOrInfer(fn.Outputs, fn.Body)
	return b.eventNode(target.Name, fn.Params, outputs, fn.Body), true
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
	case *ast.FunctionExpr:
		inputs := map[string]any{"params": e.Params}
		if e.ReturnType != "" {
			inputs["returnType"] = e.ReturnType
		}
		if len(e.Outputs) > 0 {
			inputs["outputs"] = e.Outputs
		}
		return map[string]any{"op": "FunctionLiteral", "inputs": inputs, "body": b.lowerStatements(e.Body, lowerContext{outputs: e.Outputs})}
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
	if alias, ok := protoFluxAlias(path); ok {
		return b.lowerProtoFluxCall(alias, call.Args)
	}
	if lowered, ok := b.lowerStdlibCall(path, call.Args); ok {
		return lowered
	}
	if isConstructor(path) {
		return map[string]any{"op": "Construct", "type": path, "args": args}
	}
	if member, ok := call.Callee.(*ast.MemberExpr); ok && member.Method {
		if lowered, ok := b.lowerProtoFluxMethod(member, call.Args); ok {
			return lowered
		}
		return map[string]any{"op": "MethodCall", "receiver": b.lowerExpr(member.Object), "method": member.Name, "args": args}
	}
	return map[string]any{"op": "Call", "callee": b.lowerExpr(call.Callee), "args": args}
}

func (b *Builder) lowerStdlibCall(path string, args []ast.Expr) (map[string]any, bool) {
	nodePath, ok := stdlibProtoFluxPath(path)
	if !ok {
		return nil, false
	}
	return map[string]any{
		"op":   "ProtoFluxStdlib",
		"name": path,
		"path": nodePath,
		"args": b.lowerArgs(args),
	}, true
}

func stdlibProtoFluxPath(path string) (string, bool) {
	switch path {
	case "math.abs":
		return "Operators.ValueAbs", true
	case "math.min":
		return "Operators.ValueMin", true
	case "math.max":
		return "Operators.ValueMax", true
	case "math.floor":
		return "Math.Floor", true
	case "math.ceil":
		return "Math.Ceil", true
	case "math.sqrt":
		return "Math.Sqrt", true
	case "math.sin":
		return "Math.Trigonometry.Sin", true
	case "math.cos":
		return "Math.Trigonometry.Cos", true
	case "math.tan":
		return "Math.Trigonometry.Tan", true
	case "math.rad":
		return "Math.Trigonometry.Deg2Rad", true
	case "math.deg":
		return "Math.Trigonometry.Rad2Deg", true
	case "string.len":
		return "Strings.StringLength", true
	case "string.sub":
		return "Strings.Substring", true
	case "string.find":
		return "Strings.IndexOfString", true
	case "string.format":
		return "Strings.FormatString", true
	case "table.insert":
		return "Lists.Insert", true
	case "table.remove":
		return "Lists.Remove", true
	default:
		return "", false
	}
}

func protoFluxAlias(path string) (string, bool) {
	switch path {
	case "root":
		return "pf.root", true
	case "this":
		return "pf.this", true
	case "slot":
		return "pf.slot", true
	case "node":
		return "pf.node", true
	case "source":
		return "pf.source", true
	case "ref", "reference":
		return "pf.ref", true
	case "write":
		return "pf.write", true
	case "drive":
		return "pf.drive", true
	case "debug_log":
		return "pf.debug_log", true
	default:
		return "", false
	}
}

func (b *Builder) lowerProtoFluxMethod(member *ast.MemberExpr, args []ast.Expr) (map[string]any, bool) {
	receiver := b.lowerExpr(member.Object)
	loweredArgs := b.lowerArgs(args)
	withReceiver := func(op string, names ...string) map[string]any {
		inputs := map[string]any{"op": op}
		if len(names) > 0 {
			inputs[names[0]] = receiver
		}
		for i := 1; i < len(names); i++ {
			if i-1 < len(loweredArgs) {
				inputs[names[i]] = loweredArgs[i-1]
			}
		}
		if len(loweredArgs) > len(names)-1 {
			inputs["extraArgs"] = loweredArgs[len(names)-1:]
		}
		return inputs
	}

	if dynPath, ok := dynamicVariablePath(member.Object, b); ok {
		switch member.Name {
		case "read":
			return mapWithArgs("DynamicVariableInput", []string{"path", "type", "options"}, append([]any{dynPath}, loweredArgs...)), true
		case "read_events", "input_events":
			return mapWithArgs("DynamicVariableInputWithEvents", []string{"path", "type", "options"}, append([]any{dynPath}, loweredArgs...)), true
		case "write":
			return mapWithArgs("WriteDynamicVariable", []string{"path", "value", "options"}, append([]any{dynPath}, loweredArgs...)), true
		case "create":
			return mapWithArgs("CreateDynamicVariable", []string{"path", "initialValue", "options"}, append([]any{dynPath}, loweredArgs...)), true
		case "write_or_create":
			return mapWithArgs("WriteOrCreateDynamicVariable", []string{"path", "value", "options"}, append([]any{dynPath}, loweredArgs...)), true
		case "delete":
			return mapWithArgs("DeleteDynamicVariable", []string{"path", "type"}, append([]any{dynPath}, loweredArgs...)), true
		case "drive":
			return mapWithArgs("DynamicVariableDriver", []string{"path", "field", "options"}, append([]any{dynPath}, loweredArgs...)), true
		}
	}

	switch member.Name {
	case "find", "find_slot":
		return withReceiver("FindSlot", "source", "path"), true
	case "child":
		return withReceiver("ChildSlot", "source", "name"), true
	case "parent":
		return withReceiver("ParentSlot", "source"), true
	case "children":
		return withReceiver("Children", "source"), true
	case "component":
		return withReceiver("ComponentRef", "slot", "type", "options"), true
	case "components":
		return withReceiver("ComponentList", "slot", "type", "options"), true
	case "add_component":
		return withReceiver("AddComponent", "slot", "type", "options"), true
	case "slot", "get_slot":
		return withReceiver("GetSlot", "component"), true
	case "remove":
		return withReceiver("RemoveComponent", "component"), true
	case "enabled":
		return withReceiver("ComponentEnabledSource", "component"), true
	case "set_enabled":
		return withReceiver("SetComponentEnabled", "component", "enabled"), true
	case "set_active":
		return withReceiver("SetActive", "target", "active"), true
	case "destroy":
		return withReceiver("Destroy", "target"), true
	case "source", "get":
		return withReceiver("FieldSource", "target"), true
	case "ref", "reference":
		return withReceiver("FieldReference", "target"), true
	}
	return nil, false
}

func mapWithArgs(op string, names []string, args []any) map[string]any {
	inputs := map[string]any{"op": op}
	for i, name := range names {
		if i < len(args) {
			inputs[name] = args[i]
		}
	}
	if len(args) > len(names) {
		inputs["extraArgs"] = args[len(names):]
	}
	return inputs
}

func dynamicVariablePath(expr ast.Expr, b *Builder) (any, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	ident, ok := call.Callee.(*ast.Identifier)
	if !ok || ident.Name != "dyn" || len(call.Args) == 0 {
		return nil, false
	}
	return b.lowerExpr(call.Args[0]), true
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
		return pfNodeCall(args, b)
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

func pfNodeCall(args []ast.Expr, b *Builder) map[string]any {
	inputs := pfCall("ProtoFluxNode", []string{"path", "inputs", "globals", "options"}, args, b)
	if len(args) == 0 {
		return inputs
	}
	literal, ok := args[0].(*ast.Literal)
	if !ok || literal.Kind != "string" {
		return inputs
	}
	resolved := protoflux.Resolve(literal.Value)
	if resolved.Path == "" {
		return inputs
	}
	inputs["path"] = map[string]any{"op": "Const", "kind": "string", "value": resolved.Path}
	inputs["resolvedPath"] = resolved.Path
	inputs["canonicalPath"] = resolved.Canonical
	inputs["nodeName"] = resolved.Name
	inputs["nodeCategory"] = resolved.Category
	inputs["knownNode"] = resolved.Known
	if resolved.Known {
		if node, ok := protoflux.Lookup(resolved.Canonical); ok {
			inputs["nodeInputs"] = append([]string(nil), node.Inputs...)
			inputs["nodeOutputs"] = append([]string(nil), node.Outputs...)
		}
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

func optimizeProgram(program Program) Program {
	program.Nodes = optimizeNodes(program.Nodes)
	return program
}

func optimizeNodes(nodes []Node) []Node {
	out := make([]Node, 0, len(nodes))
	for _, node := range nodes {
		node.Inputs = optimizeInputs(node.Inputs)
		node.Body = optimizeNodes(node.Body)
		node.Else = optimizeNodes(node.Else)
		out = append(out, node)
	}
	return out
}

func optimizeInputs(inputs map[string]any) map[string]any {
	if len(inputs) == 0 {
		return inputs
	}
	out := make(map[string]any, len(inputs))
	for key, value := range inputs {
		out[key] = optimizeValue(value)
	}
	return out
}

func optimizeValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			v[key] = optimizeValue(child)
		}
		if folded, ok := foldExpression(v); ok {
			return folded
		}
		return v
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, optimizeValue(item))
		}
		return out
	default:
		return value
	}
}

func foldExpression(expr map[string]any) (map[string]any, bool) {
	op, _ := expr["op"].(string)
	left, hasLeft := constNumber(expr["left"])
	right, hasRight := constNumber(expr["right"])
	if hasLeft && hasRight {
		switch op {
		case "Add":
			return numberConst(left + right), true
		case "Subtract":
			return numberConst(left - right), true
		case "Multiply":
			return numberConst(left * right), true
		case "Divide":
			if right != 0 {
				return numberConst(left / right), true
			}
		case "Modulo":
			if right != 0 {
				return numberConst(float64(int64(left) % int64(right))), true
			}
		case "Equal":
			return boolConst(left == right), true
		case "NotEqual":
			return boolConst(left != right), true
		case "LessThan":
			return boolConst(left < right), true
		case "LessThanOrEqual":
			return boolConst(left <= right), true
		case "GreaterThan":
			return boolConst(left > right), true
		case "GreaterThanOrEqual":
			return boolConst(left >= right), true
		}
	}
	if op == "Negate" {
		if value, ok := constNumber(expr["right"]); ok {
			return numberConst(-value), true
		}
	}
	return nil, false
}

func constNumber(value any) (float64, bool) {
	expr, ok := value.(map[string]any)
	if !ok {
		return 0, false
	}
	op, _ := expr["op"].(string)
	kind, _ := expr["kind"].(string)
	raw, _ := expr["value"].(string)
	if op != "Const" || kind != "number" {
		return 0, false
	}
	valueFloat, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return valueFloat, true
}

func numberConst(value float64) map[string]any {
	return map[string]any{
		"op":    "Const",
		"kind":  "number",
		"value": strconv.FormatFloat(value, 'f', -1, 64),
	}
}

func boolConst(value bool) map[string]any {
	if value {
		return map[string]any{"op": "Const", "kind": "boolean", "value": "true"}
	}
	return map[string]any{"op": "Const", "kind": "boolean", "value": "false"}
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
