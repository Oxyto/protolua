package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"protolua/internal/ast"
	"protolua/internal/backend"
	"protolua/internal/ir"
	"protolua/internal/lexer"
	"protolua/internal/lsp"
	"protolua/internal/parser"
	"protolua/internal/semantic"
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
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: protolua check [--profile protolua-extended|lua-compatible] <file>")
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
	diagnostics := semantic.Analyze(program)
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
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: protolua compile <file> [-format ir|record|brson] [-o output]")
	}
	sourcePath := fs.Arg(0)
	program, err := parseFile(sourcePath)
	if err != nil {
		return err
	}
	if err := failOnSemanticErrors(program); err != nil {
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

func parseFile(path string) (*ast.Program, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseSource(string(source))
}

func parseSource(source string) (*ast.Program, error) {
	tokens, err := lexer.New(source).Lex()
	if err != nil {
		return nil, err
	}
	return parser.Parse(tokens)
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

func failOnSemanticErrors(program *ast.Program) error {
	diagnostics := semantic.Analyze(program)
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
	fmt.Fprintln(os.Stderr, "usage: protolua <check|ast|compile|inspect-brson|lsp> <file> [options]")
}
