package parser

import (
	"fmt"
	"strings"

	"protolua/internal/ast"
	"protolua/internal/lexer"
)

type Parser struct {
	tokens []lexer.Token
	pos    int
}

func Parse(tokens []lexer.Token) (*ast.Program, error) {
	p := &Parser{tokens: tokens}
	return p.program()
}

func (p *Parser) program() (*ast.Program, error) {
	var stmts []ast.Stmt
	for !p.isAtEnd() {
		if p.matchPunct(";") {
			continue
		}
		stmt, err := p.statement()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, stmt)
	}
	return &ast.Program{Statements: stmts}, nil
}

func (p *Parser) statement() (ast.Stmt, error) {
	switch {
	case p.matchKeyword("local"):
		return p.localStmt()
	case p.matchKeyword("function"):
		return p.functionStmt()
	case p.matchKeyword("on"):
		return p.eventStmt()
	case p.matchKeyword("if"):
		return p.ifStmt()
	case p.matchKeyword("while"):
		return p.whileStmt()
	case p.matchKeyword("for"):
		return p.forStmt()
	case p.matchKeyword("return"):
		return p.returnStmt()
	case p.matchKeyword("write"):
		return p.writeStmt()
	case p.matchKeyword("drive"):
		return p.driveStmt()
	default:
		return p.assignmentOrExprStmt()
	}
}

func (p *Parser) localStmt() (ast.Stmt, error) {
	name, err := p.consume(lexer.Identifier, "expected local variable name")
	if err != nil {
		return nil, err
	}
	typ, err := p.typeAnnotation()
	if err != nil {
		return nil, err
	}
	var value ast.Expr
	if p.matchOp("=") {
		value, err = p.expression()
		if err != nil {
			return nil, err
		}
	}
	return &ast.LocalStmt{Name: name.Lexeme, Type: typ, Value: value}, nil
}

func (p *Parser) functionStmt() (ast.Stmt, error) {
	name, err := p.qualifiedName("expected function name")
	if err != nil {
		return nil, err
	}
	if _, err = p.consumeLexeme("(", "expected '(' after function name"); err != nil {
		return nil, err
	}
	params, err := p.params()
	if err != nil {
		return nil, err
	}
	if _, err = p.consumeLexeme(")", "expected ')' after parameters"); err != nil {
		return nil, err
	}
	returnType, err := p.typeAnnotation()
	if err != nil {
		return nil, err
	}
	body, err := p.blockUntil("end")
	if err != nil {
		return nil, err
	}
	p.advance()
	return &ast.FunctionStmt{Name: name, Params: params, ReturnType: returnType, Body: body}, nil
}

func (p *Parser) eventStmt() (ast.Stmt, error) {
	name, err := p.eventName()
	if err != nil {
		return nil, err
	}
	params := []ast.Param{}
	if p.matchPunct("(") {
		params, err = p.params()
		if err != nil {
			return nil, err
		}
		if _, err = p.consumeLexeme(")", "expected ')' after event parameters"); err != nil {
			return nil, err
		}
	}
	if _, err = p.consumeKeyword("do", "expected 'do' after event name"); err != nil {
		return nil, err
	}
	body, err := p.blockUntil("end")
	if err != nil {
		return nil, err
	}
	p.advance()
	return &ast.EventStmt{Name: name, Params: params, Body: body}, nil
}

func (p *Parser) ifStmt() (ast.Stmt, error) {
	cond, err := p.expression()
	if err != nil {
		return nil, err
	}
	if _, err = p.consumeKeyword("then", "expected 'then' after if condition"); err != nil {
		return nil, err
	}
	body, err := p.blockUntil("elseif", "else", "end")
	if err != nil {
		return nil, err
	}
	stmt := &ast.IfStmt{Branches: []ast.IfBranch{{Condition: cond, Body: body}}}
	for p.matchKeyword("elseif") {
		branchCond, err := p.expression()
		if err != nil {
			return nil, err
		}
		if _, err = p.consumeKeyword("then", "expected 'then' after elseif condition"); err != nil {
			return nil, err
		}
		branchBody, err := p.blockUntil("elseif", "else", "end")
		if err != nil {
			return nil, err
		}
		stmt.Branches = append(stmt.Branches, ast.IfBranch{Condition: branchCond, Body: branchBody})
	}
	if p.matchKeyword("else") {
		stmt.ElseBody, err = p.blockUntil("end")
		if err != nil {
			return nil, err
		}
	}
	if _, err = p.consumeKeyword("end", "expected 'end' after if block"); err != nil {
		return nil, err
	}
	return stmt, nil
}

func (p *Parser) whileStmt() (ast.Stmt, error) {
	cond, err := p.expression()
	if err != nil {
		return nil, err
	}
	if _, err = p.consumeKeyword("do", "expected 'do' after while condition"); err != nil {
		return nil, err
	}
	body, err := p.blockUntil("end")
	if err != nil {
		return nil, err
	}
	p.advance()
	return &ast.WhileStmt{Condition: cond, Body: body}, nil
}

func (p *Parser) forStmt() (ast.Stmt, error) {
	name, err := p.consume(lexer.Identifier, "expected loop variable name")
	if err != nil {
		return nil, err
	}
	if _, err = p.consumeLexeme("=", "expected '=' in numeric for"); err != nil {
		return nil, err
	}
	start, err := p.expression()
	if err != nil {
		return nil, err
	}
	if _, err = p.consumeLexeme(",", "expected ',' after for start expression"); err != nil {
		return nil, err
	}
	end, err := p.expression()
	if err != nil {
		return nil, err
	}
	var step ast.Expr
	if p.matchPunct(",") {
		step, err = p.expression()
		if err != nil {
			return nil, err
		}
	}
	if _, err = p.consumeKeyword("do", "expected 'do' after for range"); err != nil {
		return nil, err
	}
	body, err := p.blockUntil("end")
	if err != nil {
		return nil, err
	}
	p.advance()
	return &ast.ForStmt{Name: name.Lexeme, Start: start, End: end, Step: step, Body: body}, nil
}

func (p *Parser) returnStmt() (ast.Stmt, error) {
	if p.isBlockTerminator() || p.isAtEnd() {
		return &ast.ReturnStmt{}, nil
	}
	value, err := p.expression()
	if err != nil {
		return nil, err
	}
	return &ast.ReturnStmt{Value: value}, nil
}

func (p *Parser) writeStmt() (ast.Stmt, error) {
	target, value, err := p.targetValue("write")
	if err != nil {
		return nil, err
	}
	return &ast.WriteStmt{Target: target, Value: value}, nil
}

func (p *Parser) driveStmt() (ast.Stmt, error) {
	target, value, err := p.targetValue("drive")
	if err != nil {
		return nil, err
	}
	return &ast.DriveStmt{Target: target, Value: value}, nil
}

func (p *Parser) targetValue(kind string) (ast.Expr, ast.Expr, error) {
	target, err := p.expression()
	if err != nil {
		return nil, nil, err
	}
	if _, err = p.consumeLexeme("=", fmt.Sprintf("expected '=' in %s statement", kind)); err != nil {
		return nil, nil, err
	}
	value, err := p.expression()
	if err != nil {
		return nil, nil, err
	}
	return target, value, nil
}

func (p *Parser) assignmentOrExprStmt() (ast.Stmt, error) {
	expr, err := p.expression()
	if err != nil {
		return nil, err
	}
	if p.matchOp("=") {
		if !isAssignable(expr) {
			return nil, p.errorAt(p.previous(), "invalid assignment target")
		}
		value, err := p.expression()
		if err != nil {
			return nil, err
		}
		return &ast.AssignStmt{Target: expr, Value: value}, nil
	}
	return &ast.ExprStmt{Value: expr}, nil
}

func (p *Parser) blockUntil(terminators ...string) ([]ast.Stmt, error) {
	var body []ast.Stmt
	for !p.isAtEnd() && !p.checkAnyKeyword(terminators...) {
		if p.matchPunct(";") {
			continue
		}
		stmt, err := p.statement()
		if err != nil {
			return nil, err
		}
		body = append(body, stmt)
	}
	if p.isAtEnd() {
		return nil, p.errorAt(p.peek(), "unterminated block")
	}
	return body, nil
}

func (p *Parser) expression() (ast.Expr, error) { return p.parsePrecedence(1) }

var precedences = map[string]int{
	"or": 1, "and": 2,
	"==": 3, "~=": 3, "<": 3, "<=": 3, ">": 3, ">=": 3,
	"..": 4,
	"+":  5, "-": 5,
	"*": 6, "/": 6, "%": 6,
	"^": 8,
}

func (p *Parser) parsePrecedence(min int) (ast.Expr, error) {
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		op := p.peek().Lexeme
		prec, ok := precedences[op]
		if !ok || prec < min {
			break
		}
		p.advance()
		nextMin := prec + 1
		if op == "^" || op == ".." {
			nextMin = prec
		}
		right, err := p.parsePrecedence(nextMin)
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Left: left, Op: op, Right: right}
	}
	return left, nil
}

func (p *Parser) unary() (ast.Expr, error) {
	if p.matchOp("-") || p.matchKeyword("not") || p.matchOp("#") {
		op := p.previous().Lexeme
		right, err := p.unary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryExpr{Op: op, Right: right}, nil
	}
	return p.call()
}

func (p *Parser) call() (ast.Expr, error) {
	expr, err := p.primary()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.matchPunct("("):
			args, err := p.arguments(")")
			if err != nil {
				return nil, err
			}
			expr = &ast.CallExpr{Callee: expr, Args: args}
		case p.matchPunct("."):
			name, err := p.consume(lexer.Identifier, "expected member name after '.'")
			if err != nil {
				return nil, err
			}
			expr = &ast.MemberExpr{Object: expr, Name: name.Lexeme}
		case p.matchPunct(":"):
			name, err := p.consume(lexer.Identifier, "expected method name after ':'")
			if err != nil {
				return nil, err
			}
			if _, err = p.consumeLexeme("(", "expected '(' after method name"); err != nil {
				return nil, err
			}
			args, err := p.arguments(")")
			if err != nil {
				return nil, err
			}
			expr = &ast.CallExpr{Callee: &ast.MemberExpr{Object: expr, Name: name.Lexeme, Method: true}, Args: args}
		case p.matchPunct("["):
			index, err := p.expression()
			if err != nil {
				return nil, err
			}
			if _, err = p.consumeLexeme("]", "expected ']' after index"); err != nil {
				return nil, err
			}
			expr = &ast.IndexExpr{Object: expr, Index: index}
		default:
			return expr, nil
		}
	}
}

func (p *Parser) arguments(close string) ([]ast.Expr, error) {
	var args []ast.Expr
	if !p.checkLexeme(close) {
		for {
			arg, err := p.expression()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if !p.matchPunct(",") {
				break
			}
		}
	}
	if _, err := p.consumeLexeme(close, "expected '"+close+"' after arguments"); err != nil {
		return nil, err
	}
	return args, nil
}

func (p *Parser) primary() (ast.Expr, error) {
	if p.match(lexer.Number) {
		return &ast.Literal{Kind: "number", Value: p.previous().Lexeme}, nil
	}
	if p.match(lexer.String) {
		return &ast.Literal{Kind: "string", Value: p.previous().Lexeme}, nil
	}
	if p.matchKeyword("true") || p.matchKeyword("false") {
		return &ast.Literal{Kind: "boolean", Value: p.previous().Lexeme}, nil
	}
	if p.matchKeyword("nil") {
		return &ast.Literal{Kind: "nil", Value: "nil"}, nil
	}
	if p.match(lexer.Identifier) {
		return &ast.Identifier{Name: p.previous().Lexeme}, nil
	}
	if p.matchPunct("{") {
		return p.table()
	}
	if p.matchPunct("(") {
		expr, err := p.expression()
		if err != nil {
			return nil, err
		}
		if _, err = p.consumeLexeme(")", "expected ')' after expression"); err != nil {
			return nil, err
		}
		return expr, nil
	}
	return nil, p.errorAt(p.peek(), "expected expression")
}

func (p *Parser) table() (ast.Expr, error) {
	var fields []ast.TableField
	if !p.checkLexeme("}") {
		for {
			field, err := p.tableField()
			if err != nil {
				return nil, err
			}
			fields = append(fields, field)
			if !(p.matchPunct(",") || p.matchPunct(";")) {
				break
			}
			if p.checkLexeme("}") {
				break
			}
		}
	}
	if _, err := p.consumeLexeme("}", "expected '}' after table literal"); err != nil {
		return nil, err
	}
	return &ast.TableExpr{Fields: fields}, nil
}

func (p *Parser) tableField() (ast.TableField, error) {
	if p.check(lexer.Identifier) && p.peekN(1).Lexeme == "=" {
		key := p.advance().Lexeme
		p.advance()
		value, err := p.expression()
		if err != nil {
			return ast.TableField{}, err
		}
		return ast.TableField{Key: key, Value: value}, nil
	}
	if p.matchPunct("[") {
		key, err := p.expression()
		if err != nil {
			return ast.TableField{}, err
		}
		if _, err = p.consumeLexeme("]", "expected ']' after table key"); err != nil {
			return ast.TableField{}, err
		}
		if _, err = p.consumeLexeme("=", "expected '=' after table key"); err != nil {
			return ast.TableField{}, err
		}
		value, err := p.expression()
		if err != nil {
			return ast.TableField{}, err
		}
		return ast.TableField{KeyExpr: key, Value: value}, nil
	}
	value, err := p.expression()
	if err != nil {
		return ast.TableField{}, err
	}
	return ast.TableField{Value: value}, nil
}

func (p *Parser) params() ([]ast.Param, error) {
	params := []ast.Param{}
	if p.checkLexeme(")") {
		return params, nil
	}
	for {
		param, err := p.consume(lexer.Identifier, "expected parameter name")
		if err != nil {
			return nil, err
		}
		typ, err := p.typeAnnotation()
		if err != nil {
			return nil, err
		}
		params = append(params, ast.Param{Name: param.Lexeme, Type: typ})
		if !p.matchPunct(",") {
			break
		}
	}
	return params, nil
}

func (p *Parser) typeAnnotation() (string, error) {
	if !p.matchPunct(":") {
		return "", nil
	}
	return p.typeName("expected type name after ':'")
}

func (p *Parser) typeName(message string) (string, error) {
	first, err := p.consume(lexer.Identifier, message)
	if err != nil {
		return "", err
	}
	parts := []string{first.Lexeme}
	for p.matchPunct(".") {
		next, err := p.consume(lexer.Identifier, "expected type segment after '.'")
		if err != nil {
			return "", err
		}
		parts = append(parts, next.Lexeme)
	}
	return strings.Join(parts, "."), nil
}

func (p *Parser) qualifiedName(message string) (string, error) {
	first, err := p.consume(lexer.Identifier, message)
	if err != nil {
		return "", err
	}
	parts := []string{first.Lexeme}
	for p.matchPunct(".") {
		next, err := p.consume(lexer.Identifier, "expected name segment after '.'")
		if err != nil {
			return "", err
		}
		parts = append(parts, next.Lexeme)
	}
	if p.matchPunct(":") {
		next, err := p.consume(lexer.Identifier, "expected method name after ':'")
		if err != nil {
			return "", err
		}
		return strings.Join(parts, ".") + ":" + next.Lexeme, nil
	}
	return strings.Join(parts, "."), nil
}

func (p *Parser) eventName() (string, error) {
	switch {
	case p.match(lexer.Identifier), p.match(lexer.String):
		return p.previous().Lexeme, nil
	default:
		return "", p.errorAt(p.peek(), "expected event name")
	}
}

func isAssignable(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.Identifier, *ast.MemberExpr, *ast.IndexExpr:
		return true
	default:
		return false
	}
}

func (p *Parser) match(types ...lexer.TokenType) bool {
	for _, typ := range types {
		if p.check(typ) {
			p.advance()
			return true
		}
	}
	return false
}

func (p *Parser) matchKeyword(value string) bool {
	if p.checkKeyword(value) {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) matchOp(value string) bool {
	if p.checkLexeme(value) && p.peek().Type == lexer.Operator {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) matchPunct(value string) bool {
	if p.checkLexeme(value) && p.peek().Type == lexer.Punct {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) consume(typ lexer.TokenType, message string) (lexer.Token, error) {
	if p.check(typ) {
		return p.advance(), nil
	}
	return lexer.Token{}, p.errorAt(p.peek(), message)
}

func (p *Parser) consumeKeyword(value, message string) (lexer.Token, error) {
	if p.checkKeyword(value) {
		return p.advance(), nil
	}
	return lexer.Token{}, p.errorAt(p.peek(), message)
}

func (p *Parser) consumeLexeme(value, message string) (lexer.Token, error) {
	if p.checkLexeme(value) {
		return p.advance(), nil
	}
	return lexer.Token{}, p.errorAt(p.peek(), message)
}

func (p *Parser) check(typ lexer.TokenType) bool {
	return !p.isAtEnd() && p.peek().Type == typ
}

func (p *Parser) checkKeyword(value string) bool {
	return !p.isAtEnd() && p.peek().Type == lexer.Keyword && p.peek().Lexeme == value
}

func (p *Parser) checkLexeme(value string) bool {
	return !p.isAtEnd() && p.peek().Lexeme == value
}

func (p *Parser) checkAnyKeyword(values ...string) bool {
	for _, value := range values {
		if p.checkKeyword(value) {
			return true
		}
	}
	return false
}

func (p *Parser) isBlockTerminator() bool {
	return p.checkAnyKeyword("elseif", "else", "end", "until")
}

func (p *Parser) advance() lexer.Token {
	if !p.isAtEnd() {
		p.pos++
	}
	return p.previous()
}

func (p *Parser) isAtEnd() bool {
	return p.peek().Type == lexer.EOF
}

func (p *Parser) peek() lexer.Token {
	return p.tokens[p.pos]
}

func (p *Parser) peekN(offset int) lexer.Token {
	index := p.pos + offset
	if index >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[index]
}

func (p *Parser) previous() lexer.Token {
	return p.tokens[p.pos-1]
}

func (p *Parser) errorAt(tok lexer.Token, message string) error {
	return fmt.Errorf("%d:%d: %s near %q", tok.Line, tok.Column, message, tok.Lexeme)
}
