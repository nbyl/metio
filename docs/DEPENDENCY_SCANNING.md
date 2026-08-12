# Dependency and Code Scanning

Dependabot checks the repository's GitHub Actions, Go modules, web npm
dependencies, and Docker image references weekly. Dependabot pull requests use
the `dependencies` label and target `main`.

CodeQL analyzes the Go and JavaScript/TypeScript code on pushes and pull
requests targeting `main`, as well as on a weekly schedule. Results are
available in the repository's GitHub code-scanning alerts.

Maintainers should review new dependency updates and CodeQL alerts before
merging affected code. Findings should be resolved or documented when they
cannot be fixed immediately.
