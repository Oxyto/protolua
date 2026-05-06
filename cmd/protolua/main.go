package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"protolua/internal/ast"
	"protolua/internal/backend"
	"protolua/internal/ir"
	"protolua/internal/lexer"
	"protolua/internal/lsp"
	"protolua/internal/parser"
	"protolua/internal/protoflux"
	"protolua/internal/semantic"
)

var (
	requireRE   = regexp.MustCompile(`require\s*\(?\s*["']([^"']+)["']\s*\)?`)
	docParamRE  = regexp.MustCompile(`^\s*---@param\s+([A-Za-z_][A-Za-z0-9_]*)\s+([A-Za-z0-9_.*<>]+)\s*$`)
	docReturnRE = regexp.MustCompile(`^\s*---@return\s+(?:([A-Za-z_][A-Za-z0-9_]*)\s+)?([A-Za-z0-9_.*<>]+)\s*$`)
	docTypeRE   = regexp.MustCompile(`^\s*---@type\s+([A-Za-z0-9_.*<>]+)\s*$`)
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "protolua:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return errors.New("missing command")
	}
	switch args[0] {
	case "check":
		return check(args[1:])
	case "ast":
		return printAST(args[1:])
	case "compile":
		return compile(args[1:])
	case "inspect-brson":
		return inspectBRSON(args[1:])
	case "nodes":
		return nodes(args[1:])
	case "lsp":
		return lsp.Serve(os.Stdin, os.Stdout)
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func check(args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	profile := fs.String("profile", "protolua-extended", "syntax profile: protolua-extended or lua-compatible")
	strict := fs.Bool("strict", false, "reject unresolved ProtoFlux nodes, fields and ports")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: protolua check [--profile protolua-extended|lua-compatible] [--strict] <file>")
	}
	source, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	if err := validateProfile(string(source), *profile); err != nil {
		return err
	}
	program, err := parseSource(string(source))
	if err != nil {
		return err
	}
	diagnostics := semantic.AnalyzeWithOptions(program, semantic.Options{Strict: *strict})
	for _, diagnostic := range diagnostics {
		fmt.Fprintf(os.Stderr, "%s: %s\n", diagnostic.Severity, diagnostic.Message)
	}
	if semantic.HasErrors(diagnostics) {
		return errors.New("semantic check failed")
	}
	fmt.Println("ok")
	return nil
}

func printAST(args []string) error {
	fs := flag.NewFlagSet("ast", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: protolua ast <file>")
	}
	program, err := parseFile(fs.Arg(0))
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, program)
}

func compile(args []string) error {
	fs := flag.NewFlagSet("compile", flag.ContinueOnError)
	out := fs.String("o", "", "output file")
	format := fs.String("format", "auto", "output format: auto, ir, record, brson")
	pkg := fs.String("package", "", "package/root slot name")
	strict := fs.Bool("strict", false, "reject unresolved ProtoFlux nodes, fields and ports")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: protolua compile <file> [-format ir|record|brson] [-o output] [--strict]")
	}
	sourcePath := fs.Arg(0)
	program, err := parseFile(sourcePath)
	if err != nil {
		return err
	}
	if err := failOnSemanticErrors(program, semantic.Options{Strict: *strict}); err != nil {
		return err
	}
	resolvedFormat, err := resolveFormat(*format, *out)
	if err != nil {
		return err
	}
	if *out == "" {
		return writeCompileOutput(os.Stdout, resolvedFormat, program, sourcePath, *pkg)
	}
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeCompileOutput(f, resolvedFormat, program, sourcePath, *pkg)
}

func reorderFlags(args []string) []string {
	flags := []string{}
	positionals := []string{}
	valueFlags := map[string]bool{"-o": true, "-format": true, "-package": true}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			if strings.Contains(arg, "=") {
				continue
			}
			if valueFlags[arg] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positionals = append(positionals, arg)
	}
	return append(flags, positionals...)
}

func resolveFormat(format, out string) (string, error) {
	format = strings.ToLower(format)
	if format == "" || format == "auto" {
		switch strings.ToLower(filepath.Ext(out)) {
		case ".brson":
			return "brson", nil
		case ".json":
			if strings.HasSuffix(strings.ToLower(out), ".record.json") {
				return "record", nil
			}
			return "ir", nil
		default:
			return "ir", nil
		}
	}
	switch format {
	case "ir", "record", "brson":
		return format, nil
	default:
		return "", fmt.Errorf("unsupported compile format %q", format)
	}
}

func writeCompileOutput(file *os.File, format string, program *ast.Program, sourcePath, pkg string) error {
	switch format {
	case "ir":
		return writeJSON(file, ir.Lower(program))
	case "record":
		record, err := backend.Build(program, backend.Options{SourcePath: sourcePath, Package: pkg})
		if err != nil {
			return err
		}
		return backend.WriteRecordJSON(file, record)
	case "brson":
		record, err := backend.Build(program, backend.Options{SourcePath: sourcePath, Package: pkg})
		if err != nil {
			return err
		}
		return backend.WriteBRSON(file, record)
	default:
		return fmt.Errorf("unsupported compile format %q", format)
	}
}

func inspectBRSON(args []string) error {
	fs := flag.NewFlagSet("inspect-brson", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: protolua inspect-brson <file>")
	}
	f, err := os.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()
	envelope, err := backend.InspectBRSON(f)
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, map[string]any{
		"archiveType": envelope.ArchiveType,
		"document":    envelope.Document,
	})
}

func nodes(args []string) error {
	fs := flag.NewFlagSet("nodes", flag.ContinueOnError)
	search := fs.String("search", "", "search known ProtoFlux nodes")
	limit := fs.Int("limit", 80, "maximum nodes to print when listing/searching")
	asJSON := fs.Bool("json", false, "print JSON")
	wiki := fs.Bool("wiki", false, "fetch the ProtoFlux:All catalog names from the Resonite wiki API")
	wikiAPI := fs.String("wiki-api", protoflux.DefaultWikiAPI, "MediaWiki API URL for --wiki")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *wiki {
		nodes, err := protoflux.FetchWikiAll(context.Background(), *wikiAPI)
		if err != nil {
			return err
		}
		if *asJSON {
			return writeJSON(os.Stdout, nodes)
		}
		printNodes(nodes)
		return nil
	}
	if fs.NArg() > 0 {
		resolved := protoflux.Resolve(strings.Join(fs.Args(), " "))
		if *asJSON {
			return writeJSON(os.Stdout, resolved)
		}
		fmt.Printf("%s\ncanonical: %s\nknown: %t\n", resolved.Path, resolved.Canonical, resolved.Known)
		if resolved.Category != "" {
			fmt.Printf("category: %s\n", resolved.Category)
		}
		return nil
	}
	if *search != "" {
		matches := protoflux.Search(*search, *limit)
		if *asJSON {
			return writeJSON(os.Stdout, matches)
		}
		printNodes(matches)
		return nil
	}
	all := protoflux.Search("", *limit)
	if *asJSON {
		return writeJSON(os.Stdout, all)
	}
	printNodes(all)
	return nil
}

func printNodes(nodes []protoflux.Node) {
	for _, node := range nodes {
		path := node.Canonical
		if len(node.Aliases) > 0 {
			path = node.Aliases[0]
		}
		if node.Category != "" {
			fmt.Printf("%-44s %s\n", path, node.Category)
			continue
		}
		fmt.Println(path)
	}
}

func parseFile(path string) (*ast.Program, error) {
	source, err := readFileWithRequires(path, map[string]bool{})
	if err != nil {
		return nil, err
	}
	return parseSource(source)
}

func readFileWithRequires(path string, seen map[string]bool) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if seen[abs] {
		return "", nil
	}
	seen[abs] = true
	source, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	text := string(source)
	var combined strings.Builder
	for _, match := range requireRE.FindAllStringSubmatch(text, -1) {
		if len(match) != 2 {
			continue
		}
		requiredPath := resolveRequirePath(filepath.Dir(abs), match[1])
		requiredSource, err := readFileWithRequires(requiredPath, seen)
		if err != nil {
			return "", err
		}
		if requiredSource != "" {
			combined.WriteString(requiredSource)
			if !strings.HasSuffix(requiredSource, "\n") {
				combined.WriteString("\n")
			}
		}
	}
	combined.WriteString(text)
	return combined.String(), nil
}

func resolveRequirePath(base, raw string) string {
	path := raw
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	if filepath.Ext(path) == "" {
		path += ".plua"
	}
	return path
}

func parseSource(source string) (*ast.Program, error) {
	source = applyLuaDocAnnotations(source)
	tokens, err := lexer.New(source).Lex()
	if err != nil {
		return nil, err
	}
	return parser.Parse(tokens)
}

func applyLuaDocAnnotations(source string) string {
	lines := strings.Split(source, "\n")
	pendingParams := map[string]string{}
	pendingReturns := []ast.Param{}
	pendingType := ""
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if match := docParamRE.FindStringSubmatch(line); len(match) == 3 {
			pendingParams[match[1]] = match[2]
			out = append(out, line)
			continue
		}
		if match := docReturnRE.FindStringSubmatch(line); len(match) == 3 {
			name := match[1]
			if name == "" {
				name = "value"
			}
			pendingReturns = append(pendingReturns, ast.Param{Name: name, Type: match[2]})
			out = append(out, line)
			continue
		}
		if match := docTypeRE.FindStringSubmatch(line); len(match) == 2 {
			pendingType = match[1]
			out = append(out, line)
			continue
		}
		trimmed := strings.TrimSpace(line)
		if len(pendingParams) > 0 || len(pendingReturns) > 0 {
			if strings.HasPrefix(trimmed, "function ") || strings.HasPrefix(trimmed, "local function ") {
				line = annotateFunctionLine(line, pendingParams, pendingReturns)
				pendingParams = map[string]string{}
				pendingReturns = nil
			}
		}
		if pendingType != "" && strings.HasPrefix(trimmed, "local ") && !strings.HasPrefix(trimmed, "local function ") {
			line = annotateLocalLine(line, pendingType)
			pendingType = ""
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "---") {
			pendingParams = map[string]string{}
			pendingReturns = nil
			pendingType = ""
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func annotateFunctionLine(line string, paramTypes map[string]string, returns []ast.Param) string {
	open := strings.Index(line, "(")
	close := strings.LastIndex(line, ")")
	if open < 0 || close < open {
		return line
	}
	params := strings.Split(line[open+1:close], ",")
	for i, param := range params {
		trimmed := strings.TrimSpace(param)
		if trimmed == "" || trimmed == "..." || strings.Contains(trimmed, ":") {
			continue
		}
		if typ := paramTypes[trimmed]; typ != "" {
			params[i] = strings.Replace(param, trimmed, trimmed+": "+typ, 1)
		}
	}
	line = line[:open+1] + strings.Join(params, ",") + line[close:]
	close = strings.LastIndex(line, ")")
	if close < 0 {
		return line
	}
	if len(returns) == 0 || strings.Contains(line[close:], "->") {
		return line
	}
	suffix := ""
	if len(returns) == 1 && returns[0].Name == "value" {
		suffix = " -> " + returns[0].Type
	} else {
		parts := make([]string, 0, len(returns))
		for _, ret := range returns {
			parts = append(parts, ret.Name+": "+ret.Type)
		}
		suffix = " -> (" + strings.Join(parts, ", ") + ")"
	}
	return line[:close+1] + suffix + line[close+1:]
}

func annotateLocalLine(line, typ string) string {
	trimmed := strings.TrimSpace(line)
	afterLocal := strings.TrimSpace(strings.TrimPrefix(trimmed, "local "))
	if afterLocal == "" || strings.Contains(strings.Split(afterLocal, "=")[0], ":") || strings.Contains(strings.Split(afterLocal, "=")[0], ",") {
		return line
	}
	fields := strings.FieldsFunc(afterLocal, func(r rune) bool { return r == '=' || unicodeSpace(r) })
	if len(fields) == 0 {
		return line
	}
	name := fields[0]
	if name == "" {
		return line
	}
	return strings.Replace(line, name, name+": "+typ, 1)
}

func unicodeSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

func validateProfile(source, profile string) error {
	switch profile {
	case "", "protolua-extended":
		return nil
	case "lua-compatible":
	default:
		return fmt.Errorf("unsupported syntax profile %q", profile)
	}
	tokens, err := lexer.New(source).Lex()
	if err != nil {
		return err
	}
	for i, tok := range tokens {
		if tok.Type == lexer.Keyword {
			switch tok.Lexeme {
			case "on", "output":
				return fmt.Errorf("%d:%d: %q is ProtoLua extended syntax; use Lua-compatible events/returns instead", tok.Line, tok.Column, tok.Lexeme)
			case "write", "drive":
				if i+1 < len(tokens) && tokens[i+1].Lexeme == "(" {
					continue
				}
				return fmt.Errorf("%d:%d: %q assignment syntax is ProtoLua extended syntax; use %s(field, value)", tok.Line, tok.Column, tok.Lexeme, tok.Lexeme)
			}
		}
		if tok.Lexeme == "->" {
			return fmt.Errorf("%d:%d: output arrow is ProtoLua extended syntax; use return tables or Lua type comments", tok.Line, tok.Column)
		}
	}
	return nil
}

func failOnSemanticErrors(program *ast.Program, options semantic.Options) error {
	diagnostics := semantic.AnalyzeWithOptions(program, options)
	if !semantic.HasErrors(diagnostics) {
		return nil
	}
	return errors.New(semantic.Format(diagnostics))
}

func writeJSON(file *os.File, value any) error {
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: protolua <check|ast|compile|inspect-brson|nodes|lsp> [options]")
}
