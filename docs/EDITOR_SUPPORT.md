# Support editeurs

Le serveur de langage est integre au binaire:

```sh
protolua lsp
```

Il parle LSP sur stdio et fournit:

- diagnostics lexer/parser;
- diagnostics semantiques de base;
- completions mots-cles, types et `pf.*`;
- hover pour les primitives principales;
- semantic tokens pour la coloration VSCode/Zed.

## VSCode

Extension locale:

```sh
cd editors/vscode/protolua
npm install
npm install -g @vscode/vsce
vsce package
```

Ensuite installer le `.vsix`.

La configuration `protolua.serverPath` permet de choisir le chemin du binaire.

## Zed

Extension locale:

```sh
zed: install dev extension
```

Choisir `editors/zed/protolua`.

Le binaire `protolua` doit etre disponible dans le `PATH`. La coloration utilise Tree-sitter et les semantic tokens du LSP.
