package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"protolua/internal/ast"
	"protolua/internal/ir"
	"protolua/internal/lexer"
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
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: protolua compile <file> [-o out.pfir.json]")
	}
	program, err := parseFile(fs.Arg(0))
	if err != nil {
		return err
	}
	lowered := ir.Lower(program)
	if *out == "" {
		return writeJSON(os.Stdout, lowered)
	}
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer f.Close()
	return writeJSON(f, lowered)
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
	fmt.Fprintln(os.Stderr, "usage: protolua <check|ast|compile> <file> [options]")
}
