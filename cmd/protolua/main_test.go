package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFileIncludesRequiredModules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shared.plua"), []byte("function shared_value()\n  return 1\nend\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(dir, "main.plua")
	if err := os.WriteFile(mainPath, []byte("require(\"shared\")\non start do\n  local value = shared_value()\nend\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	program, err := parseFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Statements) != 3 {
		t.Fatalf("expected required module statements plus main file, got %d", len(program.Statements))
	}
}

func TestParseSourceAppliesLuaDocAnnotations(t *testing.T) {
	program, err := parseSource(`
---@param value float
---@param threshold float
---@return passed bool
---@return delta float
function evaluate(value, threshold)
  return value >= threshold, value - threshold
end

---@type string
local label = "ready"
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Statements) != 2 {
		t.Fatalf("expected function and local statements, got %d", len(program.Statements))
	}
}
