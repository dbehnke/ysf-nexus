---
trigger: always_on
---

# Core Development Workflow

## Sequence of Operations (Mandatory)

1. **Planning**: Create a detailed plan. Wait for human approval before coding.
2. **TDD**: Write failing tests first. Confirm failure via `task`.
3. **Implementation**: Write minimal code to pass tests.
4. **Refactoring**: Clean code while keeping tests green.

## Review Process

Before requesting human review:

1. **SSE Analysis**: Generate a report including impact assessment, edge cases, security, and performance.
2. **Notification**: Explicitly ask for review only after the analysis is shared.

## Safety

- NEVER commit secrets.
- Use `task audit-rules` periodically to check compliance.
