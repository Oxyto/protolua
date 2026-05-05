package ast

type Program struct {
	Statements []Stmt `json:"statements"`
}

type Node interface {
	node()
}

type Stmt interface {
	Node
	stmt()
}

type Expr interface {
	Node
	expr()
}

type Param struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

type LocalStmt struct {
	Name   string  `json:"name"`
	Type   string  `json:"type,omitempty"`
	Value  Expr    `json:"value,omitempty"`
	Names  []Param `json:"names,omitempty"`
	Values []Expr  `json:"values,omitempty"`
}

func (*LocalStmt) node() {}
func (*LocalStmt) stmt() {}

type AssignStmt struct {
	Target  Expr   `json:"target"`
	Value   Expr   `json:"value"`
	Targets []Expr `json:"targets,omitempty"`
	Values  []Expr `json:"values,omitempty"`
}

func (*AssignStmt) node() {}
func (*AssignStmt) stmt() {}

type FunctionStmt struct {
	Name       string  `json:"name"`
	Params     []Param `json:"params"`
	ReturnType string  `json:"returnType,omitempty"`
	Outputs    []Param `json:"outputs,omitempty"`
	Body       []Stmt  `json:"body"`
}

func (*FunctionStmt) node() {}
func (*FunctionStmt) stmt() {}

type LocalFunctionStmt struct {
	Name       string  `json:"name"`
	Params     []Param `json:"params"`
	ReturnType string  `json:"returnType,omitempty"`
	Outputs    []Param `json:"outputs,omitempty"`
	Body       []Stmt  `json:"body"`
}

func (*LocalFunctionStmt) node() {}
func (*LocalFunctionStmt) stmt() {}

type EventStmt struct {
	Name    string  `json:"name"`
	Params  []Param `json:"params,omitempty"`
	Outputs []Param `json:"outputs,omitempty"`
	Body    []Stmt  `json:"body"`
}

func (*EventStmt) node() {}
func (*EventStmt) stmt() {}

type IfBranch struct {
	Condition Expr   `json:"condition,omitempty"`
	Body      []Stmt `json:"body"`
}

type IfStmt struct {
	Branches []IfBranch `json:"branches"`
	ElseBody []Stmt     `json:"elseBody,omitempty"`
}

func (*IfStmt) node() {}
func (*IfStmt) stmt() {}

type WhileStmt struct {
	Condition Expr   `json:"condition"`
	Body      []Stmt `json:"body"`
}

func (*WhileStmt) node() {}
func (*WhileStmt) stmt() {}

type RepeatStmt struct {
	Body      []Stmt `json:"body"`
	Condition Expr   `json:"condition"`
}

func (*RepeatStmt) node() {}
func (*RepeatStmt) stmt() {}

type ForStmt struct {
	Name  string `json:"name"`
	Start Expr   `json:"start"`
	End   Expr   `json:"end"`
	Step  Expr   `json:"step,omitempty"`
	Body  []Stmt `json:"body"`
}

func (*ForStmt) node() {}
func (*ForStmt) stmt() {}

type BreakStmt struct{}

func (*BreakStmt) node() {}
func (*BreakStmt) stmt() {}

type ContinueStmt struct{}

func (*ContinueStmt) node() {}
func (*ContinueStmt) stmt() {}

type ReturnStmt struct {
	Values []Expr `json:"values,omitempty"`
}

func (*ReturnStmt) node() {}
func (*ReturnStmt) stmt() {}

type OutputStmt struct {
	Name  string `json:"name"`
	Value Expr   `json:"value"`
}

func (*OutputStmt) node() {}
func (*OutputStmt) stmt() {}

type WriteStmt struct {
	Target Expr `json:"target"`
	Value  Expr `json:"value"`
}

func (*WriteStmt) node() {}
func (*WriteStmt) stmt() {}

type DriveStmt struct {
	Target Expr `json:"target"`
	Value  Expr `json:"value"`
}

func (*DriveStmt) node() {}
func (*DriveStmt) stmt() {}

type ExprStmt struct {
	Value Expr `json:"value"`
}

func (*ExprStmt) node() {}
func (*ExprStmt) stmt() {}

type Identifier struct {
	Name string `json:"name"`
}

func (*Identifier) node() {}
func (*Identifier) expr() {}

type Literal struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

func (*Literal) node() {}
func (*Literal) expr() {}

type TableField struct {
	Key     string `json:"key,omitempty"`
	KeyExpr Expr   `json:"keyExpr,omitempty"`
	Value   Expr   `json:"value"`
}

type TableExpr struct {
	Fields []TableField `json:"fields"`
}

func (*TableExpr) node() {}
func (*TableExpr) expr() {}

type UnaryExpr struct {
	Op    string `json:"op"`
	Right Expr   `json:"right"`
}

func (*UnaryExpr) node() {}
func (*UnaryExpr) expr() {}

type BinaryExpr struct {
	Left  Expr   `json:"left"`
	Op    string `json:"op"`
	Right Expr   `json:"right"`
}

func (*BinaryExpr) node() {}
func (*BinaryExpr) expr() {}

type MemberExpr struct {
	Object Expr   `json:"object"`
	Name   string `json:"name"`
	Method bool   `json:"method,omitempty"`
}

func (*MemberExpr) node() {}
func (*MemberExpr) expr() {}

type IndexExpr struct {
	Object Expr `json:"object"`
	Index  Expr `json:"index"`
}

func (*IndexExpr) node() {}
func (*IndexExpr) expr() {}

type CallExpr struct {
	Callee Expr   `json:"callee"`
	Args   []Expr `json:"args"`
}

func (*CallExpr) node() {}
func (*CallExpr) expr() {}

type FunctionExpr struct {
	Params     []Param `json:"params"`
	ReturnType string  `json:"returnType,omitempty"`
	Outputs    []Param `json:"outputs,omitempty"`
	Body       []Stmt  `json:"body"`
}

func (*FunctionExpr) node() {}
func (*FunctionExpr) expr() {}
