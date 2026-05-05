package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"protolua/internal/lexer"
	"protolua/internal/parser"
	"protolua/internal/semantic"
)

var diagnosticRE = regexp.MustCompile(`^(\d+):(\d+):\s*(.*)$`)

type server struct {
	in        *bufio.Reader
	out       io.Writer
	mu        sync.Mutex
	documents map[string]string
}

type message struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type textDocumentItem struct {
	URI  string `json:"uri"`
	Text string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type versionedTextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type textDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type didChangeParams struct {
	TextDocument   versionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []textDocumentContentChangeEvent `json:"contentChanges"`
}

type didCloseParams struct {
	TextDocument versionedTextDocumentIdentifier `json:"textDocument"`
}

type textDocumentPositionParams struct {
	TextDocument versionedTextDocumentIdentifier `json:"textDocument"`
	Position     position                        `json:"position"`
}

type semanticTokensParams struct {
	TextDocument versionedTextDocumentIdentifier `json:"textDocument"`
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type diagnostic struct {
	Range    rangeValue `json:"range"`
	Severity int        `json:"severity"`
	Source   string     `json:"source"`
	Message  string     `json:"message"`
}

type rangeValue struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type completionItem struct {
	Label      string `json:"label"`
	Kind       int    `json:"kind,omitempty"`
	Detail     string `json:"detail,omitempty"`
	InsertText string `json:"insertText,omitempty"`
}

var tokenTypes = []string{
	"namespace",
	"type",
	"class",
	"enum",
	"interface",
	"struct",
	"typeParameter",
	"parameter",
	"variable",
	"property",
	"enumMember",
	"event",
	"function",
	"method",
	"macro",
	"keyword",
	"modifier",
	"comment",
	"string",
	"number",
	"regexp",
	"operator",
}

var tokenTypeIndex = func() map[string]int {
	out := make(map[string]int, len(tokenTypes))
	for i, typ := range tokenTypes {
		out[typ] = i
	}
	return out
}()

var pfCompletions = []completionItem{
	{Label: "events.start", Kind: 3, Detail: "Lua-compatible event", InsertText: "events.start = function()\n  \nend"},
	{Label: "root", Kind: 3, Detail: "Root slot alias", InsertText: "root()"},
	{Label: "this", Kind: 3, Detail: "Current slot alias", InsertText: "this()"},
	{Label: "slot", Kind: 3, Detail: "Slot reference alias", InsertText: "slot(\"/Path\")"},
	{Label: "node", Kind: 3, Detail: "Generic ProtoFlux node alias", InsertText: "node(\"Category.Node\", { })"},
	{Label: "source", Kind: 3, Detail: "Field source alias", InsertText: "source(field)"},
	{Label: "ref", Kind: 3, Detail: "Field reference alias", InsertText: "ref(field)"},
	{Label: "write", Kind: 3, Detail: "ProtoFlux Write alias", InsertText: "write(field, value)"},
	{Label: "drive", Kind: 3, Detail: "ProtoFlux Drive alias", InsertText: "drive(field, value)"},
	{Label: "dyn", Kind: 3, Detail: "Dynamic variable handle", InsertText: "dyn(\"Space.Name\")"},
	{Label: "pf.root", Kind: 3, Detail: "Root slot", InsertText: "pf.root()"},
	{Label: "pf.this", Kind: 3, Detail: "Current slot", InsertText: "pf.this()"},
	{Label: "pf.find_slot", Kind: 3, Detail: "Find child slot", InsertText: "pf.find_slot(root, \"Name\")"},
	{Label: "pf.child", Kind: 3, Detail: "Child slot", InsertText: "pf.child(slot, \"Name\")"},
	{Label: "pf.component", Kind: 3, Detail: "Component reference", InsertText: "pf.component(slot, \"FrooxEngine.Type\")"},
	{Label: "pf.add_component", Kind: 3, Detail: "Add component", InsertText: "pf.add_component(slot, \"FrooxEngine.Type\")"},
	{Label: "pf.field", Kind: 3, Detail: "Field reference", InsertText: "pf.field(component, \"Field\")"},
	{Label: "pf.source", Kind: 3, Detail: "Field source", InsertText: "pf.source(component.Field)"},
	{Label: "pf.ref", Kind: 3, Detail: "Field reference", InsertText: "pf.ref(component.Field)"},
	{Label: "pf.write", Kind: 3, Detail: "ProtoFlux Write", InsertText: "pf.write(target.Field, value)"},
	{Label: "pf.drive", Kind: 3, Detail: "ProtoFlux Drive", InsertText: "pf.drive(target.Field, value)"},
	{Label: "pf.node", Kind: 3, Detail: "Generic ProtoFlux node", InsertText: "pf.node(\"Category.Node\", { })"},
	{Label: "pf.impulse", Kind: 3, Detail: "Impulse output", InsertText: "pf.impulse(node, \"Port\")"},
	{Label: "pf.debug_log", Kind: 3, Detail: "Debug log", InsertText: "pf.debug_log(value)"},
	{Label: "pf.dyn.read", Kind: 3, Detail: "Read dynamic variable", InsertText: "pf.dyn.read(root, \"Space.Name\", \"type\")"},
	{Label: "pf.dyn.write", Kind: 3, Detail: "Write dynamic variable", InsertText: "pf.dyn.write(root, \"Space.Name\", value)"},
	{Label: "pf.dyn.write_or_create", Kind: 3, Detail: "Write or create dynamic variable", InsertText: "pf.dyn.write_or_create(root, \"Space.Name\", value)"},
}

func Serve(in io.Reader, out io.Writer) error {
	s := &server{
		in:        bufio.NewReader(in),
		out:       out,
		documents: map[string]string{},
	}
	for {
		msg, err := s.read()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if msg.Method == "" && len(msg.ID) == 0 {
			continue
		}
		if err := s.handle(msg); err != nil {
			if len(msg.ID) > 0 {
				_ = s.respondError(msg.ID, -32603, err.Error())
			}
		}
	}
}

func (s *server) read() (message, error) {
	length := -1
	for {
		line, err := s.in.ReadString('\n')
		if err != nil {
			return message{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			value, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return message{}, err
			}
			length = value
		}
	}
	if length < 0 {
		return message{}, fmt.Errorf("missing Content-Length")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(s.in, payload); err != nil {
		return message{}, err
	}
	var msg message
	if err := json.Unmarshal(payload, &msg); err != nil {
		return message{}, err
	}
	return msg, nil
}

func (s *server) handle(msg message) error {
	switch msg.Method {
	case "initialize":
		return s.respond(msg.ID, map[string]any{
			"serverInfo": map[string]any{"name": "ProtoLua LSP", "version": "0.1.0"},
			"capabilities": map[string]any{
				"textDocumentSync": 1,
				"completionProvider": map[string]any{
					"triggerCharacters": []string{".", ":", "_"},
				},
				"hoverProvider": true,
				"semanticTokensProvider": map[string]any{
					"legend": map[string]any{
						"tokenTypes":     tokenTypes,
						"tokenModifiers": []string{},
					},
					"full": true,
				},
			},
		})
	case "initialized":
		return nil
	case "shutdown":
		return s.respond(msg.ID, nil)
	case "exit":
		return io.EOF
	case "textDocument/didOpen":
		var params didOpenParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return err
		}
		s.documents[params.TextDocument.URI] = params.TextDocument.Text
		return s.publishDiagnostics(params.TextDocument.URI)
	case "textDocument/didChange":
		var params didChangeParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return err
		}
		if len(params.ContentChanges) > 0 {
			s.documents[params.TextDocument.URI] = params.ContentChanges[len(params.ContentChanges)-1].Text
		}
		return s.publishDiagnostics(params.TextDocument.URI)
	case "textDocument/didClose":
		var params didCloseParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return err
		}
		delete(s.documents, params.TextDocument.URI)
		return s.notify("textDocument/publishDiagnostics", map[string]any{"uri": params.TextDocument.URI, "diagnostics": []diagnostic{}})
	case "textDocument/completion":
		return s.respond(msg.ID, completionItems())
	case "textDocument/hover":
		var params textDocumentPositionParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return err
		}
		return s.respond(msg.ID, s.hover(params))
	case "textDocument/semanticTokens/full":
		var params semanticTokensParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return err
		}
		return s.respond(msg.ID, map[string]any{"data": s.semanticTokens(params.TextDocument.URI)})
	default:
		if len(msg.ID) > 0 {
			return s.respondError(msg.ID, -32601, "method not found")
		}
		return nil
	}
}

func (s *server) respond(id json.RawMessage, result any) error {
	return s.write(message{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *server) respondError(id json.RawMessage, code int, msg string) error {
	return s.write(message{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}})
}

func (s *server) notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return s.write(message{JSONRPC: "2.0", Method: method, Params: raw})
}

func (s *server) write(msg message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err = s.out.Write(payload)
	return err
}

func (s *server) publishDiagnostics(uri string) error {
	source := s.documents[uri]
	diagnostics := []diagnostic{}
	tokens, err := lexer.New(source).Lex()
	if err == nil {
		program, parseErr := parser.Parse(tokens)
		err = parseErr
		if err == nil {
			for _, sem := range semantic.Analyze(program) {
				diagnostics = append(diagnostics, diagnosticFromSemantic(sem))
			}
		}
	}
	if err != nil {
		diagnostics = append(diagnostics, diagnosticFromError(err))
	}
	return s.notify("textDocument/publishDiagnostics", map[string]any{"uri": uri, "diagnostics": diagnostics})
}

func diagnosticFromError(err error) diagnostic {
	msg := err.Error()
	line, col := 0, 0
	if matches := diagnosticRE.FindStringSubmatch(msg); len(matches) == 4 {
		line, _ = strconv.Atoi(matches[1])
		col, _ = strconv.Atoi(matches[2])
		line--
		col--
		msg = matches[3]
	}
	if line < 0 {
		line = 0
	}
	if col < 0 {
		col = 0
	}
	return diagnostic{
		Range: rangeValue{
			Start: position{Line: line, Character: col},
			End:   position{Line: line, Character: col + 1},
		},
		Severity: 1,
		Source:   "protolua",
		Message:  msg,
	}
}

func diagnosticFromSemantic(sem semantic.Diagnostic) diagnostic {
	severity := 2
	if sem.Severity == semantic.Error {
		severity = 1
	}
	return diagnostic{
		Range: rangeValue{
			Start: position{Line: 0, Character: 0},
			End:   position{Line: 0, Character: 1},
		},
		Severity: severity,
		Source:   "protolua",
		Message:  sem.Message,
	}
}

func completionItems() []completionItem {
	items := make([]completionItem, 0, len(lexer.Keywords())+len(pfCompletions)+10)
	keywords := lexer.Keywords()
	sort.Strings(keywords)
	for _, keyword := range keywords {
		items = append(items, completionItem{Label: keyword, Kind: 14, Detail: "ProtoLua keyword"})
	}
	items = append(items, pfCompletions...)
	for _, typ := range []string{"Slot", "Component", "User", "bool", "int", "float", "double", "string", "float2", "float3", "float4", "color", "quat"} {
		items = append(items, completionItem{Label: typ, Kind: 7, Detail: "ProtoLua type"})
	}
	return items
}

func (s *server) hover(params textDocumentPositionParams) any {
	source := s.documents[params.TextDocument.URI]
	word := wordAt(source, params.Position)
	if word == "" {
		return nil
	}
	detail := hoverText(word)
	if detail == "" {
		return nil
	}
	return map[string]any{
		"contents": map[string]any{
			"kind":  "markdown",
			"value": detail,
		},
	}
}

func hoverText(word string) string {
	switch word {
	case "on":
		return "`on name(inputs) -> (outputs) do ... end` declares a ProtoFlux entry point."
	case "output":
		return "`output name = value` writes a named output declared by `-> (...)`."
	case "write":
		return "`write(field, value)` or `write field = value` lowers to a ProtoFlux Write action."
	case "drive":
		return "`drive(field, value)` or `drive field = value` lowers to a ProtoFlux field drive."
	case "events":
		return "`events.name = function(...) ... end` declares a Lua-compatible ProtoFlux entry point."
	case "root":
		return "`root()` is the Lua-compatible alias for `pf.root()`."
	case "node":
		return "`node(path, inputs)` is the Lua-compatible alias for `pf.node(path, inputs)`."
	case "dyn":
		return "`dyn(path)` creates a Lua-compatible dynamic variable handle."
	case "pf":
		return "`pf` exposes ProtoFlux, Slot, Component, field and dynamic variable helpers."
	default:
		if strings.HasPrefix(word, "pf.") {
			return "`" + word + "` is a ProtoLua ProtoFlux intrinsic."
		}
		return ""
	}
}

func (s *server) semanticTokens(uri string) []int {
	source := s.documents[uri]
	tokens, err := lexer.New(source).Lex()
	if err != nil {
		return nil
	}
	data := []int{}
	prevLine, prevCol := 0, 0
	for i, tok := range tokens {
		if tok.Type == lexer.EOF {
			continue
		}
		typ := semanticType(tokens, i)
		typeID, ok := tokenTypeIndex[typ]
		if !ok {
			continue
		}
		line := tok.Line - 1
		col := tok.Column - 1
		deltaLine := line - prevLine
		deltaCol := col
		if deltaLine == 0 {
			deltaCol = col - prevCol
		}
		data = append(data, deltaLine, deltaCol, len([]rune(tok.Lexeme)), typeID, 0)
		prevLine, prevCol = line, col
	}
	return data
}

func semanticType(tokens []lexer.Token, index int) string {
	tok := tokens[index]
	switch tok.Type {
	case lexer.Keyword:
		if tok.Lexeme == "on" {
			return "event"
		}
		return "keyword"
	case lexer.Number:
		return "number"
	case lexer.String:
		return "string"
	case lexer.Operator, lexer.Punct:
		return "operator"
	case lexer.Identifier:
		if tok.Lexeme == "pf" {
			return "namespace"
		}
		if index > 0 && tokens[index-1].Lexeme == "." {
			return "property"
		}
		if index+1 < len(tokens) && tokens[index+1].Lexeme == "(" {
			return "function"
		}
		if isTypeName(tok.Lexeme) {
			return "type"
		}
		return "variable"
	default:
		return ""
	}
}

func isTypeName(value string) bool {
	if value == "" {
		return false
	}
	switch value {
	case "Slot", "Component", "User", "bool", "int", "float", "double", "string", "float2", "float3", "float4", "color", "quat":
		return true
	default:
		return unicode.IsUpper([]rune(value)[0])
	}
}

func wordAt(source string, pos position) string {
	lines := strings.Split(source, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return ""
	}
	line := []rune(lines[pos.Line])
	if pos.Character < 0 || pos.Character > len(line) {
		return ""
	}
	start := pos.Character
	for start > 0 && isWordRune(line[start-1]) {
		start--
	}
	end := pos.Character
	for end < len(line) && isWordRune(line[end]) {
		end++
	}
	if start > 0 && line[start-1] == '.' {
		prefixStart := start - 1
		for prefixStart > 0 && isWordRune(line[prefixStart-1]) {
			prefixStart--
		}
		start = prefixStart
	}
	return string(line[start:end])
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '.'
}

func EncodeRequest(method string, id int, params any) ([]byte, error) {
	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(message{JSONRPC: "2.0", ID: json.RawMessage(strconv.Itoa(id)), Method: method, Params: rawParams})
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "Content-Length: %d\r\n\r\n", len(payload))
	out.Write(payload)
	return out.Bytes(), nil
}
