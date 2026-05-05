package lexer

import (
	"fmt"
	"strings"
	"unicode"
)

type TokenType string

const (
	EOF        TokenType = "EOF"
	Identifier TokenType = "Identifier"
	Number     TokenType = "Number"
	String     TokenType = "String"
	Operator   TokenType = "Operator"
	Punct      TokenType = "Punct"
	Keyword    TokenType = "Keyword"
)

type Token struct {
	Type   TokenType `json:"type"`
	Lexeme string    `json:"lexeme"`
	Line   int       `json:"line"`
	Column int       `json:"column"`
}

var keywords = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true,
	"end": true, "false": true, "for": true, "function": true, "if": true,
	"in": true, "local": true, "nil": true, "not": true, "or": true,
	"repeat": true, "return": true, "then": true, "true": true, "until": true,
	"while": true, "on": true, "write": true, "drive": true,
}

type Lexer struct {
	source []rune
	pos    int
	line   int
	col    int
}

func New(source string) *Lexer {
	return &Lexer{source: []rune(source), line: 1, col: 1}
}

func (l *Lexer) Lex() ([]Token, error) {
	var tokens []Token
	for {
		tok, err := l.nextToken()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
		if tok.Type == EOF {
			return tokens, nil
		}
	}
}

func (l *Lexer) nextToken() (Token, error) {
	l.skipWhitespaceAndComments()
	startLine, startCol := l.line, l.col
	if l.atEnd() {
		return Token{Type: EOF, Lexeme: "", Line: startLine, Column: startCol}, nil
	}

	ch := l.advance()
	if isIdentStart(ch) {
		text := l.readWhile(func(r rune) bool { return isIdentPart(r) }, string(ch))
		if keywords[text] {
			return Token{Type: Keyword, Lexeme: text, Line: startLine, Column: startCol}, nil
		}
		return Token{Type: Identifier, Lexeme: text, Line: startLine, Column: startCol}, nil
	}

	if unicode.IsDigit(ch) {
		return Token{Type: Number, Lexeme: l.readNumber(ch), Line: startLine, Column: startCol}, nil
	}

	if ch == '"' || ch == '\'' {
		text, err := l.readString(ch)
		if err != nil {
			return Token{}, fmt.Errorf("%d:%d: %w", startLine, startCol, err)
		}
		return Token{Type: String, Lexeme: text, Line: startLine, Column: startCol}, nil
	}

	two := string([]rune{ch, l.peek()})
	if isTwoCharOperator(two) {
		l.advance()
		return Token{Type: Operator, Lexeme: two, Line: startLine, Column: startCol}, nil
	}
	if strings.ContainsRune("+-*/%^#=<>", ch) {
		return Token{Type: Operator, Lexeme: string(ch), Line: startLine, Column: startCol}, nil
	}
	if strings.ContainsRune("()[]{}.,;:", ch) {
		return Token{Type: Punct, Lexeme: string(ch), Line: startLine, Column: startCol}, nil
	}

	return Token{}, fmt.Errorf("%d:%d: unexpected character %q", startLine, startCol, ch)
}

func (l *Lexer) skipWhitespaceAndComments() {
	for !l.atEnd() {
		ch := l.peek()
		if unicode.IsSpace(ch) {
			l.advance()
			continue
		}
		if ch == '-' && l.peekNext() == '-' {
			for !l.atEnd() && l.peek() != '\n' {
				l.advance()
			}
			continue
		}
		return
	}
}

func (l *Lexer) readNumber(first rune) string {
	text := string(first)
	text = l.readWhile(func(r rune) bool { return unicode.IsDigit(r) }, text)
	if l.peek() == '.' && unicode.IsDigit(l.peekNext()) {
		text += string(l.advance())
		text = l.readWhile(func(r rune) bool { return unicode.IsDigit(r) }, text)
	}
	return text
}

func (l *Lexer) readString(quote rune) (string, error) {
	var b strings.Builder
	for !l.atEnd() {
		ch := l.advance()
		if ch == quote {
			return b.String(), nil
		}
		if ch == '\\' {
			if l.atEnd() {
				return "", fmt.Errorf("unterminated escape")
			}
			switch esc := l.advance(); esc {
			case 'n':
				b.WriteRune('\n')
			case 't':
				b.WriteRune('\t')
			case '\\', '"', '\'':
				b.WriteRune(esc)
			default:
				return "", fmt.Errorf("unsupported escape \\%c", esc)
			}
			continue
		}
		b.WriteRune(ch)
	}
	return "", fmt.Errorf("unterminated string")
}

func (l *Lexer) readWhile(fn func(rune) bool, prefix string) string {
	var b strings.Builder
	b.WriteString(prefix)
	for !l.atEnd() && fn(l.peek()) {
		b.WriteRune(l.advance())
	}
	return b.String()
}

func (l *Lexer) advance() rune {
	ch := l.source[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *Lexer) peek() rune {
	if l.atEnd() {
		return 0
	}
	return l.source[l.pos]
}

func (l *Lexer) peekNext() rune {
	if l.pos+1 >= len(l.source) {
		return 0
	}
	return l.source[l.pos+1]
}

func (l *Lexer) atEnd() bool {
	return l.pos >= len(l.source)
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r)
}

func isTwoCharOperator(s string) bool {
	switch s {
	case "==", "~=", "<=", ">=", "..":
		return true
	default:
		return false
	}
}
