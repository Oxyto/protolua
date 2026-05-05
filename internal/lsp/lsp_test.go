package lsp

import (
	"bytes"
	"strings"
	"testing"
)

func TestInitializeAndDiagnostics(t *testing.T) {
	initialize, err := EncodeRequest("initialize", 1, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	open, err := EncodeRequest("textDocument/didOpen", 2, map[string]any{
		"textDocument": map[string]any{
			"uri":  "file:///test.plua",
			"text": "on start do\n  pf.debug_log(\"ok\")\nend\n",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var input bytes.Buffer
	input.Write(initialize)
	input.Write(open)
	var output bytes.Buffer
	if err := Serve(&input, &output); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, "ProtoLua LSP") {
		t.Fatalf("initialize response missing server name: %s", got)
	}
	if !strings.Contains(got, "signatureHelpProvider") {
		t.Fatalf("initialize response missing signature help capability: %s", got)
	}
	if !strings.Contains(got, "textDocument/publishDiagnostics") {
		t.Fatalf("diagnostics notification missing: %s", got)
	}
}

func TestSignatureHelp(t *testing.T) {
	initialize, err := EncodeRequest("initialize", 1, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	open, err := EncodeRequest("textDocument/didOpen", 2, map[string]any{
		"textDocument": map[string]any{
			"uri":  "file:///test.plua",
			"text": "on start do\n  pf.node(\"Actions.Write\", )\nend\n",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	help, err := EncodeRequest("textDocument/signatureHelp", 3, map[string]any{
		"textDocument": map[string]any{"uri": "file:///test.plua"},
		"position":     map[string]any{"line": 1, "character": 27},
	})
	if err != nil {
		t.Fatal(err)
	}

	var input bytes.Buffer
	input.Write(initialize)
	input.Write(open)
	input.Write(help)
	var output bytes.Buffer
	if err := Serve(&input, &output); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if !strings.Contains(got, "pf.node(arg1, arg2?, arg3?, arg4?)") {
		t.Fatalf("signature help response missing pf.node signature: %s", got)
	}
}
