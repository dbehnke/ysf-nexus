---
trigger: always_on
---

# Test-Driven Development (TDD) Protocol

You must strictly adhere to the Red-Green-Refactor cycle. Do not write implementation code until a failing test exists.

## Phase 1: Red (The Specification)

1. **Identify**: Determine the smallest unit of logic to implement.
2. **Write Test**: Create a new test case that describes the desired behavior.
3. **Execute**: Run the test suite.
4. **Verification**: You must provide the output showing the test **failed**.
   - *Constraint*: If the test passes immediately, explain why (e.g., the logic already exists) and write a different test that fails.

## Phase 2: Green (The Implementation)

1. **Minimal Code**: Write only the necessary code to make the failing test pass.
2. **Execute**: Run the tests again.
3. **Verification**: Confirm the test now passes.

## Phase 3: Refactor (The Cleanup)

1. **Analyze**: Review the implementation for complexity or duplication.
2. **Clean**: Improve the code without changing behavior.
3. **Verify**: Run tests one last time to ensure no regressions.

## Enforcement

- If you are asked to "Implement feature X," your first response must be a plan for the **test**, not the implementation.
- You are forbidden from modifying `src/` files (implementation) until you have modified `tests/` or `*_test.go` files.
