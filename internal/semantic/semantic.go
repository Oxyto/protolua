package semantic

import (
	"fmt"
	"strings"

	"protolua/internal/ast"
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

type Analyzer struct {
	diagnostics []Diagnostic
	functions   map[string]bool
}

type scope struct {
	parent  *scope
	symbols map[string]bool
}

type context struct {
	outputs      map[string]bool
	outputOrder  []string
	outputWrites map[string]bool
	inFunction   bool
}

type pfSignature struct {
	min int
	max int
}

var builtins = map[string]bool{
	"pf":      true,
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

func Analyze(program *ast.Program) []Diagnostic {
	a := &Analyzer{functions: map[string]bool{}}
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
	return &scope{parent: parent, symbols: map[string]bool{}}
}

func (s *scope) define(name string) bool {
	if name == "" {
		return true
	}
	if s.symbols[name] {
		return false
	}
	s.symbols[name] = true
	return true
}

func (s *scope) has(name string) bool {
	if name == "" {
		return true
	}
	if s.symbols[name] {
		return true
	}
	if s.parent != nil {
		return s.parent.has(name)
	}
	return false
}

func (a *Analyzer) statements(stmts []ast.Stmt, current *scope, ctx *context) {
	for _, stmt := range stmts {
		a.statement(stmt, current, ctx)
	}
}

func (a *Analyzer) statement(stmt ast.Stmt, current *scope, ctx *context) {
	switch s := stmt.(type) {
	case *ast.LocalStmt:
		a.expr(s.Value, current)
		if !current.define(s.Name) {
			a.add(Error, fmt.Sprintf("variable %q is already declared in this scope", s.Name), s.Name)
		}
	case *ast.AssignStmt:
		if ident, ok := s.Target.(*ast.Identifier); ok {
			if !current.has(ident.Name) {
				a.add(Error, fmt.Sprintf("assignment to undeclared variable %q", ident.Name), ident.Name)
			}
		} else {
			a.expr(s.Target, current)
		}
		a.expr(s.Value, current)
	case *ast.FunctionStmt:
		fnScope := newScope(current)
		for _, param := range s.Params {
			if !fnScope.define(param.Name) {
				a.add(Error, fmt.Sprintf("parameter %q is already declared", param.Name), param.Name)
			}
		}
		ctx := newContext(s.Outputs, true)
		a.statements(s.Body, fnScope, ctx)
		a.checkUnassignedOutputs(ctx)
	case *ast.EventStmt:
		eventScope := newScope(current)
		for _, param := range s.Params {
			if !eventScope.define(param.Name) {
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
	case *ast.ForStmt:
		a.expr(s.Start, current)
		a.expr(s.End, current)
		a.expr(s.Step, current)
		forScope := newScope(current)
		forScope.define(s.Name)
		a.statements(s.Body, forScope, ctx)
	case *ast.ReturnStmt:
		for _, value := range s.Values {
			a.expr(value, current)
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
		ctx.outputWrites[s.Name] = true
	case *ast.WriteStmt:
		a.expr(s.Target, current)
		a.expr(s.Value, current)
	case *ast.DriveStmt:
		a.expr(s.Target, current)
		a.expr(s.Value, current)
	case *ast.ExprStmt:
		a.expr(s.Value, current)
	}
}

func newContext(outputs []ast.Param, inFunction bool) *context {
	ctx := &context{
		outputs:      map[string]bool{},
		outputWrites: map[string]bool{},
		inFunction:   inFunction,
	}
	for _, output := range outputs {
		ctx.outputs[output.Name] = true
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
	case *ast.IndexExpr:
		a.expr(e.Object, current)
		a.expr(e.Index, current)
	case *ast.CallExpr:
		a.call(e, current)
	}
}

func (a *Analyzer) call(call *ast.CallExpr, current *scope) {
	if path := strings.Join(callPath(call.Callee), "."); strings.HasPrefix(path, "pf.") {
		a.checkProtoFluxArity(path, len(call.Args))
	} else {
		a.expr(call.Callee, current)
	}
	for _, arg := range call.Args {
		a.expr(arg, current)
	}
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
