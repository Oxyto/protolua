# ProtoLua Syntax

ProtoLua is an imperative syntax inspired by Lua 5.4 and Luau, with a `pf` namespace for expressing ProtoFlux, Slots, Components, fields, drivers and dynamic variable interactions.

The goal is to keep the code close to Lua while producing an IR that is easy to lower into a ProtoFlux graph and then into `.brson`.

## Principles

- Control flow is textual and imperative: `if`, `while`, `for`, `function`, `return`.
- Values follow Lua primitives: numbers, strings, booleans and `nil`.
- Type annotations are optional and serve the ProtoFlux backend: `Slot`, `Component`, `float3`, `color`, `FrooxEngine.UIX.Text`.
- Explicit ProtoFlux operations can go through `pf.*`, but Lua-compatible aliases are preferred for common cases.
- Component fields can be read as sources, written once, driven continuously, or referenced.
- An unknown `pf.*` call remains representable by the generic `ProtoFluxIntrinsic` IR, which allows using a ProtoFlux node not yet specialized in the compiler.

## Syntax Profiles

ProtoLua accepts two profiles:

- `protolua-extended`: full syntax with `on`, `output`, `write field = value`, `drive field = value` and short `: Type` annotations.
- `lua-compatible`: a surface that stays close to standard Lua. Events are declared via `events.name = function(...) ... end`, writes/drives are `write(field, value)` and `drive(field, value)` calls, and named outputs can be returned with a table.

Lua-compatible profile check:

```sh
protolua check --profile lua-compatible examples/lua_compatible.plua
```

Lua-compatible prelude:

```lua
events.start = function()
  local root = root()
  local ui = root:find("UI")
  local text = ui:child("Label"):component("FrooxEngine.UIX.Text")

  write(text.Content, "Ready")
  drive(text.Color, color(0.2, 0.8, 1.0, 1.0))
  dyn("ProtoLua.Status"):write("Ready")

  node("Actions.Write", {
    Variable = text.Content:ref(),
    Value = "Generic",
  })
end

events.evaluate = function(value, threshold)
  return {
    passed = value >= threshold,
    delta = value - threshold,
  }
end
```

`pf.*` remains available as a low-level escape hatch when a simple helper does not exist yet.

Strict mode:

```sh
protolua check --strict script.plua
protolua compile script.plua --strict -format record
```

In permissive mode, an unknown node or port remains representable in the IR and emits at most a warning. In strict mode, unknown ProtoFlux nodes, unknown `pf.*` intrinsics, unknown `pf.node` ports and unrecognized options become errors.

Target Lua subset:

- Included: local variables, assignments, functions, simple closures, `if`, `while`, `repeat`, numeric `for`, `return`, calls, `:` method calls, simple tables, short/long Lua comments, `require("file")`, and a small `math`/`string`/`table` surface.
- ProtoLua-specific: `on`, `output`, short `: Type` annotations, `write field = value`, `drive field = value`, `pf` namespace.
- Not targeted for now: metatables, coroutines, arbitrary Lua IO/filesystem, fully mutable global environment, Lua debug library.

## Program

A `.plua` file is a sequence of statements:

```lua
local root: Slot = pf.root()

function clamp01(value: float): float
  if value < 0 then
    return 0
  elseif value > 1 then
    return 1
  end
  return value
end
```

Single output with `->`:

```lua
function is_visible(slot: Slot) -> bool
  return pf.source(slot.ActiveSelf)
end
```

Multiple outputs:

```lua
function divmod(value: int, divisor: int) -> (quotient: int, remainder: int)
  return value / divisor, value % divisor
end
```

Functions are lowered into `Function` IR blocks. The backend can materialize them as groups of nodes, locals and ProtoFlux continuations.

## Modules

`require("relative/path")` is recognized as a compile-time import by the CLI. The required file is read once, before the current file, and the `.plua` extension is added if it is missing:

```lua
-- shared.plua
function clamp01(value)
  return math.max(0, math.min(1, value))
end

-- main.plua
require("shared")

events.start = function()
  local amount = clamp01(2)
end
```

This form is intended for static ProtoLua helpers. Lua modules with a returned table and a full runtime loader are not part of the current subset.

## Conditions

```lua
if enabled then
  write indicator.Enabled = true
elseif debugMode then
  pf.debug_log("indicator disabled")
else
  write indicator.Enabled = false
end
```

## Loops

While loop:

```lua
while count < 10 do
  count = count + 1
end
```

Numeric loop:

```lua
for i = 1, 8 do
  pf.debug_log(i)
end

for i = 10, 1, -1 do
  pf.debug_log(i)
end
```

## Events and Impulses

`on` describes a ProtoFlux entry point:

```lua
on start do
  pf.debug_log("started")
end

on update(deltaTime: float, frameIndex: int) do
  drive spinner.Rotation = quat(0, deltaTime, 0, 1)
end
```

Multiple inputs are declared like function parameters. Multiple outputs use `-> (...)` and are populated with `output`:

```lua
on evaluate(value: float, threshold: float) -> (passed: bool, delta: float) do
  output passed = value >= threshold
  output delta = value - threshold
end
```

The event name can be a string if the backend needs to preserve an exact Resonite name:

```lua
on "OnUserJoined" do
  pf.debug_log("user joined")
end
```

## Expressions

Primitives:

```lua
nil
true
false
123
3.14
"text"
```

Operators:

```lua
a + b
a - b
a * b
a / b
a % b
a ^ b
a .. b
a == b
a ~= b
a < b
a <= b
a > b
a >= b
not enabled
```

Recognized constructors in the IR:

```lua
float2(1, 2)
float3(0, 1, 0)
float4(0, 1, 0, 1)
color(0.1, 0.8, 1.0, 1.0)
quat(0, 0, 0, 1)
type("FrooxEngine.Slot")
```

Simple Lua tables for options and named inputs:

```lua
{ direct = true, nonPersistent = false }
{ X = 1, Y = 2, Z = 3 }
```

Small Lua stdlib lowered to ProtoFlux expressions:

```lua
local a = math.abs(value)
local b = math.max(a, 1)
local c = math.floor(b)
local label = string.format("value: {0}", c)
local length = string.len(label)
table.insert(items, c)
```

Recognized functions: `math.abs`, `math.min`, `math.max`, `math.floor`, `math.ceil`, `math.sqrt`, `math.sin`, `math.cos`, `math.tan`, `math.rad`, `math.deg`, `string.len`, `string.sub`, `string.find`, `string.format`, `table.insert`, `table.remove`.

## Slots

```lua
local root = pf.root()
local self = pf.this()
local ui = pf.find_slot(root, "UI")
local labelSlot = pf.child(ui, "Label")
local parent = pf.parent(labelSlot)
local children = pf.children(ui)
```

Creation and destruction:

```lua
local created = pf.create_slot(root, "RuntimeLabel", { persistent = false })
pf.set_active(created, true)
pf.destroy(created)
```

`pf.slot(path)` represents a slot reference known by path or project identifier:

```lua
local avatarRoot = pf.slot("/User/Avatar")
```

## Components

Read or search:

```lua
local text = pf.component(labelSlot, "FrooxEngine.UIX.Text")
local renderers = pf.components(root, "FrooxEngine.UIX.Text")
local ownerSlot = pf.get_slot(text)
```

Add and remove:

```lua
local text = pf.add_component(labelSlot, "FrooxEngine.UIX.Text", {
  persistent = true,
})

pf.remove_component(text)
```

Activation:

```lua
local enabled = pf.enabled(text)
pf.set_enabled(text, true)
```

## Fields: source, write, drive, reference

ProtoFlux exposes three classic ways to interact with a Slot or Component field:

```lua
local content = pf.source(text.Content)     -- readable source
local contentRef = pf.ref(text.Content)     -- field reference
write text.Content = "Ready"                -- one-shot write
drive text.Color = color(0.1, 0.8, 1, 1)   -- continuous drive
```

Equivalent functional forms:

```lua
pf.write(text.Content, "Ready")
pf.drive(text.Color, color(1, 1, 1, 1))
pf.reference(text.Content)
pf.field(text, "Content")
pf.get(text.Content)
pf.ref_to_output(pf.ref(text.Content))
```

Practical rule:

- `write` is for one-shot modifications, like the ProtoFlux Write node.
- `drive` is for values maintained every frame.
- reading `text.Content` in a data expression is inferred as a source.
- the same `text.Content` passed to a known reference port, for example `Actions.Write.Variable`, is inferred as a reference.
- `pf.source` forces a field value read.
- `pf.ref` forces a field reference for nodes that expect a reference.
- `pf.ref_to_output` converts a ProtoFlux reference into a variable usable by indirect writes.

Field lists:

```lua
local items = pf.field_list(component, "Items")
local count = pf.list.count(items)
local first = pf.list.get(items, 0)

pf.list.set(items, 0, first)
pf.list.add(items, newItem)
pf.list.insert(items, 1, newItem)
pf.list.remove(items, 0)
pf.list.clear(items)
```

## Dynamic Variables

Spaces:

```lua
pf.dyn.space(root, "ProtoLua")
```

Read:

```lua
local title = pf.dyn.read(root, "ProtoLua.Title", "string")
local sameSpaceTitle = pf.dyn.input("ProtoLua.Title", "string")
local titleWithEvents = pf.dyn.input_events("ProtoLua.Title", "string")
```

Write:

```lua
pf.dyn.write(root, "ProtoLua.Title", "Ready")
```

Creation:

```lua
pf.dyn.create(root, "ProtoLua.Count", 0, {
  direct = true,
  nonPersistent = false,
})
```

Write-or-create:

```lua
pf.dyn.write_or_create(root, "ProtoLua.Title", "Ready", {
  direct = true,
  nonPersistent = false,
})
```

Deletion and cleanup:

```lua
pf.dyn.delete(root, "ProtoLua.Title", "string")
pf.dyn.clear(root)
pf.dyn.clear_type(root, "string")
```

Drive a dynvar into a field:

```lua
pf.dyn.drive(root, "ProtoLua.Title", text.Content)
```

## Generic ProtoFlux Nodes

To cover a node not yet exposed by a dedicated helper:

```lua
local value = pf.node("Operators.ValueAdd<float>", {
  A = 1,
  B = 2,
})

pf.node("Actions.Write", {
  Variable = pf.ref(text.Content),
  Value = "Hello",
})
```

The first argument is resolved permissively:

- `Write`, `ProtoFlux:Write` and `Actions.Write` point to the known node `ProtoFlux:Write`.
- `node("Community.Custom.Node", { ... })` remains valid even if the bundled catalog does not know it yet.
- Known nodes receive `canonicalPath`, `knownNode = true`, as well as catalogued inputs/outputs in the IR/backend.
- Unknown nodes keep their raw path with `knownNode = false`, which makes it possible to address community nodes or newer Resonite nodes without waiting for a compiler release.

The CLI exposes the same resolver:

```sh
protolua nodes Write
protolua nodes -search websocket -limit 5
protolua nodes --wiki --json
```

`--wiki` queries the MediaWiki `Category:ProtoFlux:All` API and produces a list of canonical `ProtoFlux:*` names. That automatic list gives names, but the exact ports/types/generics still have to come from more detailed Resonite sources.

Impulse calls:

```lua
pf.impulse(writer, "Write")
pf.delay(0.25)
pf.debug_log("done")
```

Packing:

```lua
pf.pack(root, { name = "ProtoLuaGraph" })
```
