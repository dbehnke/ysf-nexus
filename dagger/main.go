// YSF Nexus Dagger module for CI/CD pipeline
//
// This module provides containerized CI/CD functions for the YSF Nexus project.
// It includes functions for testing, linting, vulnerability scanning, and building
// the YSF Nexus application in reproducible containers.
//
// Functions include:
// - Test: Run Go tests
// - Lint: Run golangci-lint
// - Vuln: Run govulncheck for vulnerability scanning
// - Build: Build the YSF Nexus binary
// - CI: Run the complete CI pipeline (test, lint, vuln check)

package main

import (
	"context"
	"dagger/ysf-nexus/internal/dagger"
)

type YsfNexus struct{}

// Base returns a Go container with the YSF Nexus source code mounted
func (m *YsfNexus) Base(source *dagger.Directory) *dagger.Container {
	return dag.Container().
		From("golang:1.25").
		WithMountedDirectory("/src", source).
		WithWorkdir("/src")
}

// Test runs all Go tests in the YSF Nexus project
func (m *YsfNexus) Test(ctx context.Context, source *dagger.Directory) (string, error) {
	return m.Base(source).
		WithExec([]string{"go", "test", "./..."}).
		Stdout(ctx)
}

// Lint runs golangci-lint on the YSF Nexus project
func (m *YsfNexus) Lint(ctx context.Context, source *dagger.Directory) (string, error) {
	return m.Base(source).
		// Use the project's Makefile to install and run linting tools so behavior
		// matches CI and local development (`make install-tools` installs pinned
		// tool versions, `make lint` runs golangci-lint).
<
		WithExec([]string{"make", "install-tools"}).
		// Run verify-tools (prints versions only when TOOLS_DEBUG=1) and then run lint.
		WithExec([]string{"sh", "-lc", "export PATH=/usr/local/go/bin:/go/bin:$HOME/go/bin:$PATH && TOOLS_DEBUG=${TOOLS_DEBUG:-} make verify-tools && make lint"}).
		Stdout(ctx)
}

// Vuln runs govulncheck on the YSF Nexus project
func (m *YsfNexus) Vuln(ctx context.Context, source *dagger.Directory) (string, error) {
	return m.Base(source).
		// Install dev tools via Makefile (includes govulncheck) then run the
		// vulnerability scanner from the module root.
		WithExec([]string{"make", "install-tools"}).

		// Run verify-tools (prints versions only when TOOLS_DEBUG=1) then run govulncheck.
		WithExec([]string{"sh", "-lc", "export PATH=/usr/local/go/bin:/go/bin:$HOME/go/bin:$PATH && TOOLS_DEBUG=${TOOLS_DEBUG:-} make verify-tools && govulncheck ./..."}).
		Stdout(ctx)
}

// Build builds the YSF Nexus binary
func (m *YsfNexus) Build(source *dagger.Directory) *dagger.File {
	return m.Base(source).
		WithExec([]string{"go", "build", "-o", "ysf-nexus", "./cmd/ysf-nexus"}).
		File("/src/ysf-nexus")
}

// CI runs the complete CI pipeline (test, lint, vuln check)
func (m *YsfNexus) CI(ctx context.Context, source *dagger.Directory) (string, error) {
	// Run tests
	if _, err := m.Test(ctx, source); err != nil {
		return "", err
	}

	// Run linter
	if _, err := m.Lint(ctx, source); err != nil {
		return "", err
	}

	// Run vulnerability check
	if _, err := m.Vuln(ctx, source); err != nil {
		return "", err
	}

	return "CI pipeline completed successfully", nil
}
