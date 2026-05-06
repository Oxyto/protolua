package semantic

import (
	"fmt"
	"strings"

	"protolua/internal/ast"
	"protolua/internal/protoflux"
)

const (
	Error   = "error"
	Warning = "warning"
)

type Diagnostic struct {
	Severity string
	Message  string
	Symbol   string
}

type Options struct {
	Strict bool
}

type Analyzer struct {
	diagnostics []Diagnostic
	functions   map[string]bool
	options     Options
}

type scope struct {
	parent  *scope
	symbols map[string]string
}

type context struct {
	outputs      map[string]bool
	outputTypes  map[string]string
	outputOrder  []string
	outputWrites map[string]bool
	inFunction   bool
}

type pfSignature struct {
	min int
	max int
}

type ProtoFluxSignature struct {
	Path string
	Min  int
	Max  int
}

var builtins = map[string]bool{
	"pf":      true,
	"events":  true,
	"root":    true,
	"this":    true,
	"slot":    true,
	"node":    true,
	"source":  true,
	"ref":     true,
	"dyn":     true,
	"write":   true,
	"drive":   true,
	"color":   true,
	"float2":  true,
	"float3":  true,
	"float4":  true,
	"quat":    true,
	"int":     true,
	"float":   true,
	"double":  true,
	"string":  true,
	"bool":    true,
	"Vector2": true,
	"Vector3": true,
	"Vector4": true,
	"math":    true,
	"table":   true,
	"require": true,
}

var pfSignatures = map[string]pfSignature{
	"pf.root":                {min: 0, max: 0},
	"pf.this":                {min: 0, max: 0},
	"pf.slot":                {min: 1, max: 1},
	"pf.find_slot":           {min: 2, max: 2},
	"pf.child":               {min: 2, max: 2},
	"pf.parent":              {min: 1, max: 1},
	"pf.children":            {min: 1, max: 1},
	"pf.create_slot":         {min: 2, max: 3},
	"pf.destroy":             {min: 1, max: 1},
	"pf.set_active":          {min: 2, max: 2},
	"pf.component":           {min: 2, max: 3},
	"pf.components":          {min: 2, max: 3},
	"pf.get_slot":            {min: 1, max: 1},
	"pf.add_component":       {min: 2, max: 3},
	"pf.remove_component":    {min: 1, max: 1},
	"pf.enabled":             {min: 1, max: 1},
	"pf.set_enabled":         {min: 2, max: 2},
	"pf.field":               {min: 2, max: 2},
	"pf.field_list":          {min: 2, max: 2},
	"pf.get":                 {min: 1, max: 1},
	"pf.source":              {min: 1, max: 2},
	"pf.reference":           {min: 1, max: 2},
	"pf.ref":                 {min: 1, max: 2},
	"pf.ref_to_output":       {min: 1, max: 1},
	"pf.as_variable":         {min: 1, max: 1},
	"pf.write":               {min: 2, max: 2},
	"pf.drive":               {min: 2, max: 2},
	"pf.list.get":            {min: 2, max: 2},
	"pf.list.count":          {min: 1, max: 1},
	"pf.list.set":            {min: 3, max: 3},
	"pf.list.add":            {min: 2, max: 2},
	"pf.list.insert":         {min: 3, max: 3},
	"pf.list.remove":         {min: 2, max: 2},
	"pf.list.clear":          {min: 1, max: 1},
	"pf.node":                {min: 1, max: 4},
	"pf.impulse":             {min: 2, max: 2},
	"pf.pack":                {min: 1, max: 2},
	"pf.unpack":              {min: 1, max: 1},
	"pf.delay":               {min: 1, max: 1},
	"pf.debug_log":           {min: 1, max: 1},
	"pf.dyn.read":            {min: 3, max: 3},
	"pf.dyn.write":           {min: 3, max: 3},
	"pf.dyn.create":          {min: 3, max: 4},
	"pf.dyn.write_or_create": {min: 3, max: 4},
	"pf.dyn.delete":          {min: 3, max: 3},
	"pf.dyn.clear":           {min: 1, max: 1},
	"pf.dyn.clear_type":      {min: 2, max: 2},
	"pf.dyn.input":           {min: 2, max: 3},
	"pf.dyn.input_events":    {min: 2, max: 3},
	"pf.dyn.space":           {min: 2, max: 3},
	"pf.dyn.drive":           {min: 3, max: 4},
}

var stdlibSignatures = map[string]pfSignature{
	"math.abs":      {min: 1, max: 1},
	"math.min":      {min: 2, max: 2},
	"math.max":      {min: 2, max: 2},
	"math.floor":    {min: 1, max: 1},
	"math.ceil":     {min: 1, max: 1},
	"math.sqrt":     {min: 1, max: 1},
	"math.sin":      {min: 1, max: 1},
	"math.cos":      {min: 1, max: 1},
	"math.tan":      {min: 1, max: 1},
	"math.rad":      {min: 1, max: 1},
	"math.deg":      {min: 1, max: 1},
	"string.len":    {min: 1, max: 1},
	"string.sub":    {min: 2, max: 3},
	"string.find":   {min: 2, max: 2},
	"string.format": {min: 1, max: 32},
	"table.insert":  {min: 2, max: 3},
	"table.remove":  {min: 1, max: 2},
}

func Analyze(program *ast.Program) []Diagnostic {
	return AnalyzeWithOptions(program, Options{})
}

func AnalyzeWithOptions(program *ast.Program, options Options) []Diagnostic {
	a := &Analyzer{functions: map[string]bool{}, options: options}
	root := newScope(nil)
	for name := range builtins {
		root.define(name)
	}
	for _, stmt := range program.Statements {
		if fn, ok := stmt.(*ast.FunctionStmt); ok {
			a.functions[fn.Name] = true
			root.define(fn.Name)
		}
	}
	a.statements(program.Statements, root, nil)
	return a.diagnostics
}

func HasErrors(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == Error {
			return true
		}
	}
	return false
}

func Format(diagnostics []Diagnostic) string {
	lines := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		lines = append(lines, fmt.Sprintf("%s: %s", diagnostic.Severity, diagnostic.Message))
	}
	return strings.Join(lines, "\n")
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent, symbols: map[string]string{}}
}

func (s *scope) define(name string, typ ...string) bool {
	if name == "" {
		return true
	}
	if _, ok := s.symbols[name]; ok {
		return false
	}
	if len(typ) > 0 {
		s.symbols[name] = typ[0]
	} else {
		s.symbols[name] = ""
	}
	return true
}

func (s *scope) has(name string) bool {
	if name == "" {
		return true
	}
	if _, ok := s.symbols[name]; ok {
		return true
	}
	if s.parent != nil {
		return s.parent.has(name)
	}
	return false
}

func (s *scope) typeOf(name string) string {
	if name == "" {
		return ""
	}
	if typ, ok := s.symbols[name]; ok {
		return typ
	}
	if s.parent != nil {
		return s.parent.typeOf(name)
	}
	return ""
}

func (s *scope) assignType(name, typ string) {
	if name == "" || typ == "" {
		return
	}
	if _, ok := s.symbols[name]; ok {
		s.symbols[name] = typ
		return
	}
	if s.parent != nil && s.parent.has(name) {
		s.parent.assignType(name, typ)
	}
}

func (a *Analyzer) statements(stmts []ast.Stmt, current *scope, ctx *context) {
	for _, stmt := range stmts {
		a.statement(stmt, current, ctx)
	}
}

func (a *Analyzer) statement(stmt ast.Stmt, current *scope, ctx *context) {
	switch s := stmt.(type) {
	case *ast.LocalStmt:
		values := s.Values
		if len(values) == 0 && s.Value != nil {
			values = []ast.Expr{s.Value}
		}
		for _, value := range values {
			a.expr(value, current)
		}
		names := s.Names
		if len(names) == 0 {
			names = []ast.Param{{Name: s.Name, Type: s.Type}}
		}
		for i, name := range names {
			typ := name.Type
			if i < len(values) {
				valueType := a.inferExprType(values[i], current)
				if typ == "" {
					typ = valueType
				} else {
					a.checkAssignableType(typ, valueType, fmt.Sprintf("local %q", name.Name), name.Name)
					if isGenericComponentType(typ) && isSpecificComponentType(valueType) {
						typ = valueType
					}
				}
			}
			if !current.define(name.Name, typ) {
				a.add(Error, fmt.Sprintf("variable %q is already declared in this scope", name.Name), name.Name)
			}
		}
	case *ast.AssignStmt:
		targets := s.Targets
		values := s.Values
		if len(targets) == 0 {
			targets = []ast.Expr{s.Target}
		}
		if len(values) == 0 && s.Value != nil {
			values = []ast.Expr{s.Value}
		}
		for i, target := range targets {
			if ident, ok := target.(*ast.Identifier); ok {
				if !current.has(ident.Name) {
					a.add(Error, fmt.Sprintf("assignment to undeclared variable %q", ident.Name), ident.Name)
				} else if i < len(values) {
					valueType := a.inferExprType(values[i], current)
					existing := current.typeOf(ident.Name)
					a.checkAssignableType(existing, valueType, fmt.Sprintf("assignment to %q", ident.Name), ident.Name)
					if existing == "" {
						current.assignType(ident.Name, valueType)
					}
				}
			} else {
				a.expr(target, current)
			}
		}
		for _, value := range values {
			a.expr(value, current)
		}
	case *ast.FunctionStmt:
		fnScope := newScope(current)
		for _, param := range s.Params {
			if !fnScope.define(param.Name, param.Type) {
				a.add(Error, fmt.Sprintf("parameter %q is already declared", param.Name), param.Name)
			}
		}
		ctx := newContext(s.Outputs, true)
		a.statements(s.Body, fnScope, ctx)
		a.checkUnassignedOutputs(ctx)
	case *ast.LocalFunctionStmt:
		if !current.define(s.Name) {
			a.add(Error, fmt.Sprintf("function %q is already declared in this scope", s.Name), s.Name)
		}
		fnScope := newScope(current)
		for _, param := range s.Params {
			if !fnScope.define(param.Name, param.Type) {
				a.add(Error, fmt.Sprintf("parameter %q is already declared", param.Name), param.Name)
			}
		}
		ctx := newContext(s.Outputs, true)
		a.statements(s.Body, fnScope, ctx)
		a.checkUnassignedOutputs(ctx)
	case *ast.EventStmt:
		eventScope := newScope(current)
		for _, param := range s.Params {
			if !eventScope.define(param.Name, param.Type) {
				a.add(Error, fmt.Sprintf("event input %q is already declared", param.Name), param.Name)
			}
		}
		ctx := newContext(s.Outputs, false)
		a.statements(s.Body, eventScope, ctx)
		a.checkUnassignedOutputs(ctx)
	case *ast.IfStmt:
		for _, branch := range s.Branches {
			a.expr(branch.Condition, current)
			a.statements(branch.Body, newScope(current), ctx)
		}
		a.statements(s.ElseBody, newScope(current), ctx)
	case *ast.WhileStmt:
		a.expr(s.Condition, current)
		a.statements(s.Body, newScope(current), ctx)
	case *ast.RepeatStmt:
		a.statements(s.Body, newScope(current), ctx)
		a.expr(s.Condition, current)
	case *ast.ForStmt:
		a.expr(s.Start, current)
		a.expr(s.End, current)
		a.expr(s.Step, current)
		forScope := newScope(current)
		forScope.define(s.Name, "number")
		a.statements(s.Body, forScope, ctx)
	case *ast.ReturnStmt:
		for i, value := range s.Values {
			a.expr(value, current)
			if ctx != nil && ctx.inFunction && i < len(ctx.outputOrder) {
				output := ctx.outputOrder[i]
				a.checkAssignableType(ctx.outputTypes[output], a.inferExprType(value, current), "return value", output)
			}
		}
		if ctx != nil && ctx.inFunction && len(ctx.outputOrder) > 0 {
			if len(s.Values) != len(ctx.outputOrder) {
				a.add(Error, fmt.Sprintf("return has %d value(s), expected %d named output(s)", len(s.Values), len(ctx.outputOrder)), "")
			}
			for i := range s.Values {
				if i < len(ctx.outputOrder) {
					ctx.outputWrites[ctx.outputOrder[i]] = true
				}
			}
		}
	case *ast.OutputStmt:
		a.expr(s.Value, current)
		if ctx == nil || !ctx.outputs[s.Name] {
			a.add(Error, fmt.Sprintf("output %q is not declared in this block signature", s.Name), s.Name)
			return
		}
		a.checkAssignableType(ctx.outputTypes[s.Name], a.inferExprType(s.Value, current), fmt.Sprintf("output %q", s.Name), s.Name)
		ctx.outputWrites[s.Name] = true
	case *ast.WriteStmt:
		a.expr(s.Target, current)
		a.expr(s.Value, current)
	case *ast.DriveStmt:
		a.expr(s.Target, current)
		a.expr(s.Value, current)
	case *ast.BreakStmt, *ast.ContinueStmt:
	case *ast.ExprStmt:
		a.expr(s.Value, current)
	}
}

func newContext(outputs []ast.Param, inFunction bool) *context {
	ctx := &context{
		outputs:      map[string]bool{},
		outputTypes:  map[string]string{},
		outputWrites: map[string]bool{},
		inFunction:   inFunction,
	}
	for _, output := range outputs {
		ctx.outputs[output.Name] = true
		ctx.outputTypes[output.Name] = output.Type
		ctx.outputOrder = append(ctx.outputOrder, output.Name)
	}
	return ctx
}

func (a *Analyzer) checkUnassignedOutputs(ctx *context) {
	for _, output := range ctx.outputOrder {
		if !ctx.outputWrites[output] {
			a.add(Warning, fmt.Sprintf("output %q is declared but never assigned", output), output)
		}
	}
}

func (a *Analyzer) expr(expr ast.Expr, current *scope) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		if !current.has(e.Name) && !a.functions[e.Name] {
			a.add(Error, fmt.Sprintf("use of undeclared variable %q", e.Name), e.Name)
		}
	case *ast.Literal:
	case *ast.TableExpr:
		for _, field := range e.Fields {
			a.expr(field.KeyExpr, current)
			a.expr(field.Value, current)
		}
	case *ast.UnaryExpr:
		a.expr(e.Right, current)
	case *ast.BinaryExpr:
		a.expr(e.Left, current)
		a.expr(e.Right, current)
	case *ast.MemberExpr:
		a.expr(e.Object, current)
		a.checkComponentField(e, current)
	case *ast.IndexExpr:
		a.expr(e.Object, current)
		a.expr(e.Index, current)
	case *ast.CallExpr:
		a.call(e, current)
	case *ast.FunctionExpr:
		fnScope := newScope(current)
		for _, param := range e.Params {
			if !fnScope.define(param.Name, param.Type) {
				a.add(Error, fmt.Sprintf("parameter %q is already declared", param.Name), param.Name)
			}
		}
		ctx := newContext(e.Outputs, true)
		a.statements(e.Body, fnScope, ctx)
		a.checkUnassignedOutputs(ctx)
	}
}

func (a *Analyzer) call(call *ast.CallExpr, current *scope) {
	if path := strings.Join(callPath(call.Callee), "."); strings.HasPrefix(path, "pf.") {
		a.checkProtoFluxCall(path, call)
	} else if alias, ok := semanticProtoFluxAlias(path); ok {
		a.checkProtoFluxCall(alias, call)
	} else if isStdlibPath(path) {
		a.checkStdlibCall(path, len(call.Args))
	} else {
		a.expr(call.Callee, current)
	}
	for _, arg := range call.Args {
		a.expr(arg, current)
	}
}

func isStdlibPath(path string) bool {
	return strings.HasPrefix(path, "math.") || strings.HasPrefix(path, "string.") || strings.HasPrefix(path, "table.")
}

func (a *Analyzer) checkStdlibCall(path string, got int) {
	signature, ok := stdlibSignatures[path]
	if !ok {
		if a.options.Strict {
			a.add(Error, fmt.Sprintf("unknown Lua stdlib function %q in strict mode", path), path)
		}
		return
	}
	if got < signature.min || got > signature.max {
		expected := fmt.Sprintf("%d", signature.min)
		if signature.max != signature.min {
			expected = fmt.Sprintf("%d..%d", signature.min, signature.max)
		}
		a.add(Error, fmt.Sprintf("%s expects %s argument(s), got %d", path, expected, got), path)
	}
}

func (a *Analyzer) checkProtoFluxCall(path string, call *ast.CallExpr) {
	a.checkProtoFluxArity(path, len(call.Args))
	if _, ok := pfSignatures[path]; !ok && a.options.Strict {
		a.add(Error, fmt.Sprintf("unknown ProtoFlux intrinsic %q in strict mode", path), path)
	}
	switch path {
	case "pf.node":
		a.checkProtoFluxNode(call)
	default:
		a.checkProtoFluxOptions(path, call)
	}
}

func semanticProtoFluxAlias(path string) (string, bool) {
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
	case "ref":
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

func LookupProtoFluxSignature(path string) (ProtoFluxSignature, bool) {
	if alias, ok := semanticProtoFluxAlias(path); ok {
		path = alias
	}
	signature, ok := pfSignatures[path]
	if !ok {
		return ProtoFluxSignature{}, false
	}
	return ProtoFluxSignature{Path: path, Min: signature.min, Max: signature.max}, true
}

func (a *Analyzer) checkProtoFluxArity(path string, got int) {
	signature, ok := pfSignatures[path]
	if !ok {
		return
	}
	if got < signature.min || got > signature.max {
		expected := fmt.Sprintf("%d", signature.min)
		if signature.max != signature.min {
			expected = fmt.Sprintf("%d..%d", signature.min, signature.max)
		}
		a.add(Error, fmt.Sprintf("%s expects %s argument(s), got %d", path, expected, got), path)
	}
}

func (a *Analyzer) checkProtoFluxNode(call *ast.CallExpr) {
	if len(call.Args) == 0 {
		return
	}
	literal, ok := call.Args[0].(*ast.Literal)
	if !ok || literal.Kind != "string" {
		return
	}
	resolved := protoflux.Resolve(literal.Value)
	if !resolved.Known {
		if a.options.Strict {
			a.add(Error, fmt.Sprintf("unknown ProtoFlux node %q in strict mode", literal.Value), literal.Value)
		}
		return
	}
	node, ok := protoflux.Lookup(resolved.Canonical)
	if !ok {
		return
	}
	if len(call.Args) > 1 {
		table, ok := call.Args[1].(*ast.TableExpr)
		if ok {
			allowed := portSet(node.Inputs)
			for _, field := range table.Fields {
				if field.Key == "" || allowed[field.Key] {
					continue
				}
				a.add(a.strictSeverity(), fmt.Sprintf("node %s has no input port %q", resolved.Canonical, field.Key), field.Key)
			}
		}
	}
}

func (a *Analyzer) checkProtoFluxOptions(path string, call *ast.CallExpr) {
	optionIndex := protoFluxOptionIndex(path)
	if optionIndex < 0 || optionIndex >= len(call.Args) {
		return
	}
	table, ok := call.Args[optionIndex].(*ast.TableExpr)
	if !ok {
		return
	}
	allowed := map[string]bool{
		"direct":        true,
		"nonPersistent": true,
		"persistent":    true,
		"name":          true,
		"type":          true,
		"generic":       true,
		"position":      true,
		"color":         true,
	}
	for _, field := range table.Fields {
		if field.Key == "" || allowed[field.Key] {
			continue
		}
		a.add(a.strictSeverity(), fmt.Sprintf("%s options do not define %q", path, field.Key), field.Key)
	}
}

func protoFluxOptionIndex(path string) int {
	switch path {
	case "pf.create_slot", "pf.component", "pf.components", "pf.add_component":
		return 2
	case "pf.dyn.input", "pf.dyn.input_events", "pf.dyn.space":
		return 2
	case "pf.dyn.create", "pf.dyn.write_or_create", "pf.dyn.drive":
		return 3
	case "pf.pack":
		return 1
	default:
		return -1
	}
}

func (a *Analyzer) inferExprType(expr ast.Expr, current *scope) string {
	if expr == nil {
		return ""
	}
	switch e := expr.(type) {
	case *ast.Identifier:
		return current.typeOf(e.Name)
	case *ast.Literal:
		switch e.Kind {
		case "boolean":
			return "bool"
		case "number":
			if strings.Contains(e.Value, ".") {
				return "float"
			}
			return "int"
		case "string":
			return "string"
		case "nil":
			return "nil"
		default:
			return ""
		}
	case *ast.UnaryExpr:
		right := a.inferExprType(e.Right, current)
		switch e.Op {
		case "not":
			return "bool"
		case "-", "#":
			return right
		default:
			return ""
		}
	case *ast.BinaryExpr:
		left := a.inferExprType(e.Left, current)
		right := a.inferExprType(e.Right, current)
		switch e.Op {
		case "==", "~=", "<", "<=", ">", ">=", "and", "or":
			return "bool"
		case "..":
			return "string"
		case "+", "-", "*", "/", "%", "^":
			return numericResultType(left, right)
		default:
			return ""
		}
	case *ast.TableExpr:
		return "table"
	case *ast.MemberExpr:
		objectType := a.inferExprType(e.Object, current)
		if isSpecificComponentType(objectType) {
			return componentFieldType(componentName(objectType), e.Name)
		}
		return ""
	case *ast.CallExpr:
		path := strings.Join(callPath(e.Callee), ".")
		switch path {
		case "bool":
			return "bool"
		case "int":
			return "int"
		case "float":
			return "float"
		case "double":
			return "double"
		case "string":
			return "string"
		case "color":
			return "color"
		case "float2", "Vector2":
			return "float2"
		case "float3", "Vector3":
			return "float3"
		case "float4", "Vector4":
			return "float4"
		case "quat":
			return "quat"
		case "require":
			return "module"
		case "math.abs", "math.min", "math.max", "math.sqrt", "math.sin", "math.cos", "math.tan", "math.rad", "math.deg":
			return "float"
		case "math.floor", "math.ceil":
			return "int"
		case "string.len", "string.find":
			return "int"
		case "string.sub", "string.format":
			return "string"
		case "table.insert", "table.remove":
			return "nil"
		case "root", "this", "slot", "pf.root", "pf.this", "pf.slot":
			return "Slot"
		case "pf.component":
			if len(e.Args) >= 2 {
				if literal, ok := e.Args[1].(*ast.Literal); ok && literal.Kind == "string" {
					return componentType(literal.Value)
				}
			}
			return "Component"
		default:
			if member, ok := e.Callee.(*ast.MemberExpr); ok && member.Method {
				switch member.Name {
				case "component", "add_component":
					if len(e.Args) >= 1 {
						if literal, ok := e.Args[0].(*ast.Literal); ok && literal.Kind == "string" {
							return componentType(literal.Value)
						}
					}
					return "Component"
				case "find", "find_slot", "child", "parent", "slot", "get_slot":
					return "Slot"
				}
			}
			return ""
		}
	default:
		return ""
	}
}

func numericResultType(left, right string) string {
	left = normalizeType(left)
	right = normalizeType(right)
	if left == "double" || right == "double" {
		return "double"
	}
	if left == "float" || right == "float" || left == "number" || right == "number" {
		return "float"
	}
	if left == "int" && right == "int" {
		return "int"
	}
	return ""
}

func (a *Analyzer) checkAssignableType(want, got, context, symbol string) {
	want = normalizeType(want)
	got = normalizeType(got)
	if want == "" || got == "" || got == "nil" {
		return
	}
	if want == got {
		return
	}
	if isGenericComponentType(want) && isSpecificComponentType(got) {
		return
	}
	if isSpecificComponentType(want) && isGenericComponentType(got) {
		return
	}
	if isNumericType(want) && isNumericType(got) {
		return
	}
	a.add(Error, fmt.Sprintf("%s expects %s, got %s", context, want, got), symbol)
}

func normalizeType(typ string) string {
	switch strings.ToLower(typ) {
	case "boolean":
		return "bool"
	case "number":
		return "float"
	case "integer":
		return "int"
	default:
		return typ
	}
}

func isNumericType(typ string) bool {
	switch typ {
	case "int", "float", "double":
		return true
	default:
		return false
	}
}

func (a *Analyzer) checkComponentField(expr *ast.MemberExpr, current *scope) {
	objectType := a.inferExprType(expr.Object, current)
	if !isSpecificComponentType(objectType) {
		return
	}
	component := componentName(objectType)
	if componentFieldKnown(component, expr.Name) {
		return
	}
	a.add(a.strictSeverity(), fmt.Sprintf("component %s has no known field %q", component, expr.Name), expr.Name)
}

func (a *Analyzer) strictSeverity() string {
	if a.options.Strict {
		return Error
	}
	return Warning
}

func portSet(ports []string) map[string]bool {
	out := map[string]bool{}
	for _, port := range ports {
		if port == "" || port == "*" {
			continue
		}
		out[port] = true
	}
	return out
}

func componentType(name string) string {
	if name == "" {
		return "Component"
	}
	return "Component:" + name
}

func isGenericComponentType(typ string) bool {
	return normalizeType(typ) == "Component"
}

func isSpecificComponentType(typ string) bool {
	return strings.HasPrefix(typ, "Component:")
}

func componentName(typ string) string {
	return strings.TrimPrefix(typ, "Component:")
}

func componentFieldKnown(component, field string) bool {
	fields, ok := componentFields[component]
	if !ok {
		return false
	}
	return fields[field] != ""
}

func componentFieldType(component, field string) string {
	fields, ok := componentFields[component]
	if !ok {
		return ""
	}
	return fields[field]
}

var componentFields = map[string]map[string]string{
	"FrooxEngine.UIX.Text": {
		"Content":            "string",
		"Color":              "color",
		"Size":               "float",
		"HorizontalAutoSize": "bool",
		"VerticalAutoSize":   "bool",
		"Enabled":            "bool",
	},
	"FrooxEngine.Slot": {
		"Name":       "string",
		"Tag":        "string",
		"ActiveSelf": "bool",
		"Persistent": "bool",
		"Position":   "float3",
		"Rotation":   "quat",
		"Scale":      "float3",
	},
}

func callPath(expr ast.Expr) []string {
	switch e := expr.(type) {
	case *ast.Identifier:
		return []string{e.Name}
	case *ast.MemberExpr:
		if e.Method {
			return nil
		}
		base := callPath(e.Object)
		if len(base) == 0 {
			return nil
		}
		return append(base, e.Name)
	default:
		return nil
	}
}

func (a *Analyzer) add(severity, message, symbol string) {
	a.diagnostics = append(a.diagnostics, Diagnostic{Severity: severity, Message: message, Symbol: symbol})
}
