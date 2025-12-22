# Tooling & CLI Rules

## GitHub (gh CLI)
- Use `gh` for all PRs, issues, and checks.
- **Protocol**: Always write body content to a temporary file, use `gh ... --body-file <file>`, then delete the temp file. No direct browser usage.

## Build System
- Use `Taskfile.dev` (`task` command) for all CI/Build tasks.
- **Note**: The `Makefile` is deprecated; do not use it.
