package main

import (
	"context"
	"fmt"
	"os"

	dagger "go.dagger.io/dagger/go/dagger"
)

func main() {
	ctx := context.Background()
	c, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		fmt.Fprintf(os.Stderr, "dagger connect: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()

	// Use the repository root as the build context. When this program is
	// executed from the dagger/pipeline directory (as the CI workflow does),
	// the repo root is two levels up.
	src := c.Host().Directory("../..")

	// Base image with Go installed
	golang := c.Container().From("golang:1.25")

	// Create container for running go test
	test := golang.WithMountedDirectory("/src", src).
		WithWorkdir("/src/ysf-nexus").
		WithExec([]string{"/bin/sh", "-c", "go test ./..."})

	// Run tests
	_, err = test.ExitCode(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tests failed: %v\n", err)
		os.Exit(1)
	}

	// Install golangci-lint in the container and run it
	lint := golang.WithMountedDirectory("/src", src).
		WithWorkdir("/src/ysf-nexus").
		WithExec([]string{"/bin/sh", "-c", "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest && $GOBIN/golangci-lint run ./..."})

	_, err = lint.ExitCode(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "golangci-lint failed: %v\n", err)
		os.Exit(1)
	}

	// Install govulncheck and run it
	vuln := golang.WithMountedDirectory("/src", src).
		WithWorkdir("/src/ysf-nexus").
		WithExec([]string{"/bin/sh", "-c", "go install golang.org/x/vuln/cmd/govulncheck@latest && $GOBIN/govulncheck ./..."})

	// govulncheck exits non-zero when vulnerabilities are found, which will cause the pipeline to fail
	_, err = vuln.ExitCode(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "govulncheck failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("dagger pipeline completed: tests, lint, and vulncheck passed")
}
