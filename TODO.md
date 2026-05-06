# ProtoLua TODO

Tracking list for the remaining work before ProtoLua is production-usable with Resonite.

## Priority 1: importable `.brson`

- [x] Build a stable backend `record` model from the ProtoLua IR.
- [x] Replace the experimental `PLBRSON` container with a writer that emits the Resonite `FrDT` header.
- [x] Add an internal BSON encoder with no external dependency.
- [x] Emit a root document close to the public schema: `VersionNumber`, `FeatureFlags`, `Types`, `TypeVersions`, `Object`, `Asset`.
- [x] Replace the raw BSON archive with an LZ4 frame archive (`archiveType = 1`) with no external dependency.
- [x] Validate the documented archive type: `0x01 = LZ4 frame mode`.
- [x] Read compressed LZ4 blocks in `inspect-brson` for future exported fixtures.
- [ ] Validate the LZ4 archive produced directly by Resonite import.
- [x] Encode `Components` as `UniquePtr(Array(IComponent))`.
- [x] Encode Slot `ParentReference` as `PackageNodeIdentifier`.
- [x] Add backend helpers for `UniquePtr`, `WorldElementRef`, `Sync<T>` and `SyncList<T>`.
- [ ] Validate `WorldElementRef` against a real Resonite fixture.
- [ ] Map all real component `SyncRef<T>`/`SyncList<T>` fields from fixtures.
- [x] Populate the canonical type names emitted in `Types`, for example `[FrooxEngine]FrooxEngine.Slot`.
- [x] Encode `IComponent.Type` as `ComponentTableIndex`.
- [x] Materialize the ProtoFlux graph as debug slots/components with normalized ports, data/reference wires and impulse wires.
- [x] Expose `pf.node("...", { ... })` table entries as dedicated node ports.
- [ ] Map the real ProtoFlux components with their exact persistent fields.
- [ ] Generate port, wire, impulse and drive references according to the real ProtoFlux layout.
- [ ] Import a generated file into Resonite and iterate until it validates.
- [ ] Add `.brson` fixtures exported from Resonite for compatibility tests.

## ProtoFlux Backend

- [x] Add an open resolver so `pf.node("...")`/`node("...")` can address any ProtoFlux path, including nodes missing from the bundled catalog.
- [x] Add a bundled catalog of common ProtoFlux nodes with path, category, inputs and outputs when known.
- [x] Propagate resolution metadata (`canonicalPath`, `knownNode`, catalogued ports) into the IR and backend record.
- [x] Add the `protolua nodes` command to list, search and resolve known nodes.
- [ ] Build a complete catalog of known ProtoFlux nodes: path, inputs, outputs, generic types, impulses and defaults.
- [x] Import/generate the full catalog automatically from a verified Resonite/wiki source instead of maintaining a partial list by hand.
- [ ] Map ProtoFlux generic signatures (`T`, `IWorldElement`, `SyncRef<T>`, collections, refs) onto the ProtoLua type system.
- [ ] Add an overload resolver to choose the correct ProtoFlux node based on argument types.
- [ ] Resolve `FrooxEngine.*` types and ProtoFlux generics.
- [ ] Validate component fields (`Content`, `Color`, `Enabled`, etc.).
- [ ] Lower `if`, `while`, `for`, `function`, `on`, `return`, `output` into real connected Resonite ProtoFlux nodes.
- [ ] Support control-flow continuations/impulses with branches, merges, delays, async and explicit sequencing.
- [x] Generate a normalized graph with input/output ports, `return`, `output`, dependency wires and impulse chaining.
- [ ] Place nodes and groups in a readable layout in the graph.
- [ ] Materialize `write`, `drive`, `pf.source`, `pf.ref`, dynvars and field lists with the exact Resonite nodes.
- [ ] Materialize common Resonite events (`Start`, `Update`, user events, interactions, collisions, equip, UIX) with their exact inputs.
- [ ] Support ProtoLua-to-ProtoLua function calls as reusable ProtoFlux groups/subgraphs.
- [x] Support nodes that expose multiple data outputs and multiple impulses in the normalized graph when the catalog knows the ports.
- [ ] Support useful automatic conversions to keep the syntax simple: refs, sources, values, numeric casts and inferred generics.
- [x] Add a strict mode that rejects any unresolved node/field, and a permissive mode that keeps `pf.node`/`ProtoFluxIntrinsic`.
- [x] Add a simple optimization pass: constant folding.
- [ ] Extend optimization to dead nodes and unused locals.

## Semantic Analysis

- [x] Undeclared variables.
- [x] Variables redefined in the same scope.
- [x] Wrong argument count for known `pf.*` helpers.
- [x] Declared outputs that are never assigned.
- [x] `output` to an undeclared name.
- [x] Simple type incompatibilities.
- [x] Local type inference to avoid writing annotations everywhere.
- [ ] Validate component fields according to the real component type.
- [x] Validate `pf.*` option tables and named `pf.node` ports.
- [ ] Validate multiple returns and multiple outputs across all control-flow paths.
- [ ] Diagnostics with precise source ranges.

## Language

- [x] Officially define the target Lua subset: what must stay identical to standard Lua and what is intentionally ProtoLua-specific.
- [x] Keep simple scripts readable as Lua: less mandatory `pf.*` when the type system can infer the ProtoFlux operation.
- [x] Add Lua-like syntax sugar for common ProtoFlux cases: field source/ref/write/drive, component lookup, slot lookup, dynvars.
- [x] Define two syntax profiles: `lua-compatible` with only valid Lua forms, and `protolua-extended` with the `on`, `output`, `write ... =`, `drive ... =` sugars.
- [x] Add a Lua-compatible prelude that aliases common ProtoFlux operations as Lua calls: `root()`, `this()`, `slot(path)`, `node(path, inputs)`, `source(field)`, `ref(field)`, `write(field, value)`, `drive(field, value)`.
- [x] Keep `pf.*` as a low-level escape hatch, but document the simple aliases as the main API.
- [x] Add Lua method-call syntax for slots/components: `root:find("UI")`, `slot:child("Label")`, `slot:parent()`, `slot:children()`, `slot:component("Type")`, `slot:add_component("Type", opts)`.
- [x] Add Lua method-call syntax for fields/dynvars: `field:source()`, `field:ref()`, `dyn("Space.Name"):read("string")`, `dyn("Space.Name"):write(value)`, `dyn("Space.Name"):drive(field)`.
- [x] Add a Lua-compatible alternative to `on`: an event table or standard Lua declarations, for example `events.start = function() ... end` and `events.update = function(deltaTime) ... end`.
- [x] Add a Lua-compatible alternative to `output`: `return { passed = ..., delta = ... }` for named outputs, and `return a, b` for positional outputs.
- [x] Add a Lua-compatible alternative to `: Type` annotations: type-style comments `---@param`, `---@return`, `---@type`, while keeping the short annotations in the extended mode.
- [x] Formalize the inference rules that avoid `pf.source`/`pf.ref`: field read in an expression -> source, field passed to a reference port -> ref, field assignment -> one-shot write.
- [x] Keep `drive(...)` explicit for continuous values so a standard Lua assignment does not change semantics depending on context.
- [x] Add a `check --profile lua-compatible` verification command that rejects non-standard Lua sugars.
- [x] `repeat ... until`.
- [x] `break` and `continue`.
- [x] More complete tables: numeric indices, nested fields, implicit fields, trailing separators everywhere.
- [x] Support long Lua comments `--[[ ... ]]`.
- [x] Support local functions `local function name(...) ... end`.
- [x] Support Lua-like multiple calls and assignments (`local a, b = ...`, `a, b = b, a`).
- [x] Support varargs `...` when useful for helpers and wrappers.
- [x] Method calls `slot:method()` with semantic lowering.
- [x] Modules/imports if multiple files become necessary.
- [x] Add a small Lua-compatible stdlib where it compiles cleanly to ProtoFlux: useful `math`, `string`, `table`.
- [x] Decide explicitly whether metatables, coroutines, arbitrary IO/filesystem and the Lua debug library are excluded, simulated or partially supported.
- [x] Add a set of "simple Lua to ProtoFlux" examples to measure how straightforward the syntax is.

## LSP

- [x] Syntax diagnostics.
- [x] Completion for keywords/types/`pf.*`/Lua-compatible aliases.
- [x] Semantic tokens.
- [x] Basic semantic diagnostics.
- [x] Signature help for `pf.*`.
- [x] Completion for known ProtoFlux nodes in `node("...")`/`pf.node("...")`.
- [x] Go-to-definition.
- [x] Rename symbol.
- [x] Document formatting.

## Editors

- [x] Local VSCode extension.
- [x] Local Zed extension.
- [ ] Package and test the VSCode `.vsix`.
- [ ] Test the Zed extension in the editor.
- [ ] Publish final installation instructions.

## Distribution

- [x] `CGO_ENABLED=0` builds for Linux/macOS/Windows.
- [x] Release checksums.
- [x] Test CI.
- [x] Cross-compilation CI.
