# ProtoLua VSCode Extension

Install dependencies and package normally:

```sh
npm install
npm install -g @vscode/vsce
vsce package
```

The extension starts the language server with:

```sh
protolua lsp
```

Set `protolua.serverPath` if the binary is not on `PATH`.
