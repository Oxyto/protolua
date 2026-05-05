const vscode = require("vscode");
const { LanguageClient, TransportKind } = require("vscode-languageclient/node");

let client;

function activate(context) {
  const config = vscode.workspace.getConfiguration("protolua");
  const serverPath = config.get("serverPath") || "protolua";

  client = new LanguageClient(
    "protolua",
    "ProtoLua LSP",
    {
      command: serverPath,
      args: ["lsp"],
      transport: TransportKind.stdio,
    },
    {
      documentSelector: [{ scheme: "file", language: "protolua" }],
      synchronize: {
        fileEvents: vscode.workspace.createFileSystemWatcher("**/*.plua"),
      },
    }
  );

  context.subscriptions.push(client.start());
}

function deactivate() {
  if (!client) {
    return undefined;
  }
  return client.stop();
}

module.exports = { activate, deactivate };
