package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// This runner intentionally avoids importing the Dagger Go SDK due to
// module path mismatches on the runner. Instead it runs the pipeline steps
// inside a reproducible container using docker. The CI workflow already
// exposes the docker socket to this process.
func main() {
	// When executed from dagger/pipeline (the workflow cd's there), the
	// repository root is two levels up.
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get cwd: %v\n", err)
		os.Exit(1)
	}

	repoRoot := filepath.Clean(filepath.Join(cwd, "..", ".."))

	// Build the docker run command to execute tests, lint, and govulncheck
	// inside golang:1.25. Use an explicit absolute path for the mount.
	cmd := fmt.Sprintf(
		"docker run --rm -v %s:/src -w /src/ysf-nexus golang:1.25 /bin/sh -c '%s'",
		repoRoot,
		// Command executed inside the container
		"set -e; go test ./...; go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; $GOBIN/golangci-lint run ./...; go install golang.org/x/vuln/cmd/govulncheck@latest; $GOBIN/govulncheck ./...",
	)

	fmt.Println("running:", cmd)

	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	fmt.Print(string(out))
	if err != nil {
		fmt.Fprintf(os.Stderr, "pipeline failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("pipeline completed: tests, lint, and vulncheck passed")
}
