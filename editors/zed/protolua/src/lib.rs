use zed_extension_api as zed;

struct ProtoLuaExtension;

impl zed::Extension for ProtoLuaExtension {
    fn new() -> Self {
        Self
    }

    fn language_server_command(
        &mut self,
        _language_server_id: &zed::LanguageServerId,
        worktree: &zed::Worktree,
    ) -> zed::Result<zed::Command> {
        let command = worktree
            .which("protolua")
            .ok_or_else(|| "Could not find `protolua` on PATH".to_string())?;

        Ok(zed::Command {
            command,
            args: vec!["lsp".to_string()],
            env: worktree.shell_env(),
        })
    }
}

zed::register_extension!(ProtoLuaExtension);
