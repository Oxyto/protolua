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
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: protolua check <file>")
	}
	_, err := parseFile(fs.Arg(0))
	if err != nil {
		return err
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
		return backend.WriteExperimentalBRSON(file, record)
	default:
		return fmt.Errorf("unsupported compile format %q", format)
	}
}

func parseFile(path string) (*ast.Program, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	tokens, err := lexer.New(string(source)).Lex()
	if err != nil {
		return nil, err
	}
	return parser.Parse(tokens)
}

func writeJSON(file *os.File, value any) error {
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: protolua <check|ast|compile|lsp> <file> [options]")
}
