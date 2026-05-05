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
- Lua-compatible event declarations with `events.name = function(...) ... end`
- Lua-compatible ProtoFlux aliases such as `root()`, `node(...)`, `write(field, value)`, `drive(field, value)` and `slot:component(...)`
- component/field interactions through `pf.*`, `write` and `drive`
- simple Lua table literals for named ProtoFlux inputs/options
- `repeat ... until`
- `break` and `continue`
- `local function`
- numbers, strings, booleans and `nil`
- arithmetic, comparison and boolean operators

See [SYNTAX.md](SYNTAX.md) for the language reference.

## CLI

Run a syntax check:

```sh
go run ./cmd/protolua check examples/basic.plua
```

`check` runs syntax and basic semantic diagnostics: undeclared variables, same-scope redefinitions, named output usage and known `pf.*` arities.

Validate the Lua-compatible surface:

```sh
go run ./cmd/protolua check --profile lua-compatible examples/lua_compatible.plua
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

Inspect the generated `.brson` bootstrap document:

```sh
go run ./cmd/protolua inspect-brson out.brson
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

The `.brson` writer currently emits the public `FrDT` + LZ4 frame container and a BSON document with a normalized ProtoFlux graph, including ports and wires for data, field references and impulse sequencing. Exact Resonite persistent ProtoFlux component fields are still tracked in [TODO.md](TODO.md).
