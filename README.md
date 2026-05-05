# ProtoLua

ProtoLua is a Lua/Luau-inspired imperative language intended to compile into a ProtoFlux-compatible representation.

This repository intentionally keeps the compiler dependency-free: the CLI is written in Go using only the standard library, so release builds can be shipped as one binary per platform.

## Syntax Direction

ProtoLua keeps familiar Lua/Luau statements while mapping each imperative construct to explicit ProtoFlux-style operations:

```lua
local speed = 2.5
local distance = 10

function travel(time)
  local result = speed * time
  if result > distance then
    return distance
  end
  return result
end

for i = 1, 4 do
  distance = distance + i
end
```

Supported front-end features:

- `local` variables
- assignments
- `function name(...) ... end`
- `if ... then ... elseif ... then ... else ... end`
- `while ... do ... end`
- numeric `for i = start, stop[, step] do ... end`
- `return`
- function calls
- optional type annotations
- `on ... do ... end` event entry points
- event/function inputs and named outputs with `-> (...)`
- component/field interactions through `pf.*`, `write` and `drive`
- simple Lua table literals for named ProtoFlux inputs/options
- numbers, strings, booleans and `nil`
- arithmetic, comparison and boolean operators

See [SYNTAX.md](SYNTAX.md) for the language reference.

## CLI

Run a syntax check:

```sh
go run ./cmd/protolua check examples/basic.plua
```

Compile to the current ProtoLua IR:

```sh
go run ./cmd/protolua compile examples/basic.plua -o out.pfir.json
```

Compile to the backend record model:

```sh
go run ./cmd/protolua compile examples/flux_component.plua -format record -o out.record.json
```

Compile to the experimental `.brson` carrier:

```sh
go run ./cmd/protolua compile examples/flux_component.plua -format brson -o out.brson
```

Run the editor language server:

```sh
go run ./cmd/protolua lsp
```

Print the parsed AST:

```sh
go run ./cmd/protolua ast examples/basic.plua
```

## Single Binary Builds

Build for the current platform:

```sh
go build -o protolua ./cmd/protolua
```

Cross-compile examples:

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o dist/protolua-linux-amd64 ./cmd/protolua
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o dist/protolua-windows-amd64.exe ./cmd/protolua
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o dist/protolua-darwin-arm64 ./cmd/protolua
```

The `.pfir.json` output is a stable intermediate representation. The backend record model is documented in [docs/BACKEND.md](docs/BACKEND.md), and editor support is documented in [docs/EDITOR_SUPPORT.md](docs/EDITOR_SUPPORT.md).
