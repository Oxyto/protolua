# ProtoLua Zed Extension

Install this directory as a Zed dev extension.

The extension expects the compiler binary on `PATH` and starts:

```sh
protolua lsp
```

Syntax color comes from both the bundled Tree-sitter grammar and the ProtoLua LSP semantic tokens. In Zed, semantic tokens can be enabled with:

```json
{
  "languages": {
    "ProtoLua": {
      "semantic_tokens": "combined"
    }
  }
}
```
