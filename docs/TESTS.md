Integration and unit test usage

This project contains both unit and integration (end-to-end) tests. Integration tests exercise networking and multiple components and are skipped by default in CI.

Running unit tests (default)

Run the fast unit tests (recommended for local development and CI):

```
go test ./... 
```

Running integration tests (opt-in)

Integration and E2E tests are gated behind the `integration` build tag. This prevents heavy network tests from running in default CI runs.

- Run all tests including integration tests:

```
go test -tags=integration ./...
```

- Run integration tests for a single package:

```
go test -tags=integration ./internal/testhelpers -run TestBridgeTalkerScenarios -v
```

- Run with verbose output:

```
go test -v -tags=integration ./...
```

Notes for CI

- If you want to run integration tests in CI, add a separate job that runs `go test -tags=integration ./...` on an appropriate runner (Linux with network access). You can choose to run this job on-demand (workflow_dispatch), when a PR has a label (e.g. `run-integration`), or on a schedule (nightly).

- Example GitHub Actions job (simple, on-demand):

```yaml
name: Integration Tests
on:
  workflow_dispatch: {}

jobs:
  integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.25'
      - name: Install Dagger CLI (if needed)
        run: |
          curl -fsSL https://dl.dagger.io/dagger/install.sh | sudo BIN_DIR=/usr/local/bin sh -s --
      - name: Run integration tests
        run: |
          go test -v -tags=integration ./...
```

Troubleshooting

- Integration tests may require privileged ports or network loopback access. If tests fail on GitHub Actions, consider running them on self-hosted runners or in a scheduled nightly job where you can control environment.

- If an individual integration test is flaky, add a short timeout or retry in the test, or gate it behind `testing.Short()` and skip when `-test.short` is provided.

If you'd like, I can add the GitHub Actions job above to this repository and wire it to run on `workflow_dispatch` or on a schedule. Let me know which option you prefer.
