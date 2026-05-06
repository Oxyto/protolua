# Editor Support

The language server is built into the binary:

```sh
protolua lsp
```

It speaks LSP over stdio and provides:

- lexer/parser diagnostics;
- basic semantic diagnostics;
- completions for keywords, types, `pf.*` and Lua-compatible aliases;
- hover for the main primitives;
- semantic tokens for VSCode/Zed highlighting;
- local go-to-definition;
- local symbol rename;
- simple document formatting.

## VSCode

Local extension:

```sh
cd editors/vscode/protolua
npm install
npm install -g @vscode/vsce
vsce package
```

Then install the `.vsix`.

The `protolua.serverPath` setting lets you choose the binary path.

## Zed

Local extension:

```sh
zed: install dev extension
```

Choose `editors/zed/protolua`.

The `protolua` binary must be available on `PATH`. Highlighting uses Tree-sitter and LSP semantic tokens.
