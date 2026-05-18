# commerce

ncgo micro workspace for module `github.com/x/commerce`.

- Workspace metadata: `ncgo.workspace`
- Workspace orchestration: `compose.yaml`
- Local hooks config: `.pre-commit-config.yaml`
- Services live under `services/` and keep their own `.ncgo/manifest.yaml`.

Use `ncgo add rpc <name>` to add Kitex RPC services and `ncgo add bff <name>` to add Hertz BFF services.
