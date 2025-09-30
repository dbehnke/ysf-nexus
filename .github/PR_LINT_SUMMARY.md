Lint fixes summary

Summary:
- Fixed golangci-lint findings across test helpers, mocks, and small packages.
- Addressed primarily `errcheck` (unchecked errors), `staticcheck` issues, and one unused symbol.

Files changed (high level):
- internal/testhelpers/* (bridge_talker_scenarios.go, integration_test_suite.go, mock_bridge.go, mock_network.go, mock_repeater.go, simulate_bridge_activity_test.go)
- internal/tools/* (bridge_sender/main.go, ws_client/main.go)
- pkg/bridge/* (bridge.go, manager.go)
- Makefile (add `install-tools` and update `lint` target)
- .github/workflows/golangci-lint.yml (now calls `make install-tools` and `make lint`)

Why:
- CI was failing due to `golangci-lint` issues and inconsistent tooling installs. The changes ensure checked errors are handled (or intentionally ignored) and remove deprecated/use warnings. Centralizing tooling via `Makefile` makes local development and CI consistent.

Verification:
- `golangci-lint run ./...` locally returns 0 issues.
- `make lint` runs successfully (calls `golangci-lint` that must be installed or installed via `make install-tools`).
- Changes are committed on branch `feat/enhanced-bridge-system` (commits include lint-fix and CI Makefile changes).

Suggested follow-ups:
- Optionally add a CI workflow to run integration tests behind `workflow_dispatch` or a separate protected job (they're currently gated by the `integration` Go build tag).
- Optionally add `make install-tools` to other CI workflows that require additional developer tools.

Commands to reproduce locally:
```fish
# install dev tools
make install-tools
# run linter
make lint
# run unit tests
go test ./...
# run integration tests (explicit)
go test -tags=integration ./...
```

If you'd like, I can post this summary as a PR comment or expand it into the PR body.
