# ProtoLua Backend

The backend transforms the ProtoLua AST into IR, then into a Resonite/ProtoFlux record model.

## Formats

### `ir`

Frontend IR JSON output:

```sh
protolua compile examples/flux_component.plua -format ir
```

### `record`

Readable, deterministic record model:

```sh
protolua compile examples/flux_component.plua -format record -o out.record.json
```

The record contains:

- a root slot;
- a root ProtoFlux graph component;
- the `on` entry points;
- functions;
- ProtoFlux nodes and their inputs;
- normalized data/reference/impulse ports;
- wires between inputs, locals, `output`, `return`, field references and impulse chains;
- named outputs;
- backend diagnostics/warnings.

### `brson`

Experimental binary container:

```sh
protolua compile examples/flux_component.plua -format brson -o out.brson
```

Current state:

- public `FrDT` header;
- 4 reserved bytes;
- archive type as a varbyte;
- `archiveType = 1`, corresponding to the documented LZ4 frame mode;
- valid LZ4 frame with uncompressed blocks, to keep the Go binary dependency-free;
- `inspect-brson` can also read compressed LZ4 blocks, to compare future fixtures exported by Resonite;
- root BSON document with `VersionNumber`, `FeatureFlags`, `Types`, `TypeVersions`, `Object`, `Asset`;
- `Types` contains canonical CLI-style names, and `IComponent.Type` is encoded as an index into this table;
- `Slot.Components` is encoded as `UniquePtr(Array(IComponent))`;
- slot children carry a `ParentReference` to the parent's ID;
- the ProtoFlux graph is materialized as debug slots with `ProtoLua.Runtime.*` components, normalized ports and wires;
- table entries in `pf.node("...", { ... })` become node ports (`Variable`, `Value`, etc.);
- known generic nodes from the bundled catalog carry their resolved path, canonical path and catalogued ports;
- unknown generic nodes remain materialized with their raw path, which keeps the backend open to uncatalogued Resonite nodes and community nodes;
- `ProtoLua` subdocument to keep the full debug record.

Important: the container now uses the documented LZ4 frame mode and carries a normalized connected graph, but the file is still not guaranteed to import into Resonite until the exact layout of persistent components and ProtoFlux references is validated.

Inspection:

```sh
protolua inspect-brson out.brson
```

## Automatic Resolution

`-format auto` chooses based on the extension:

- `.brson` -> `brson`
- `.record.json` -> `record`
- other or stdout -> `ir`

## Dependency-Free Backend

The backend uses only the Go standard library. It keeps the goal of shipping a single binary per platform.

## Next Step for Resonite Import

To get a `.brson` importable by Resonite, the experimental writer has to be replaced with an official Resonite Record serializer:

1. map `backend.Record` to the exact Resonite record types;
2. write components and fields with their exact Resonite IDs/types;
3. map `UniquePtr`, `WorldElementRef`, `Sync<T>` and type indices exactly;
4. encode the `.brson` binary according to the exact Resonite layout;
5. test the import in Resonite;
6. keep `record` as the stable debug format.
