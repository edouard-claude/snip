# Contributing to snip

Thanks for your interest! Here's how to get started.

## Quick Start

```bash
git clone https://github.com/edouard-claude/snip.git
cd snip
make build
make test
```

## Adding a Filter

Filters are YAML files in `filters/`. No Go knowledge required.

1. Create `filters/your-tool.yaml` following existing filters as examples
2. Add test fixtures in `tests/fixtures/`
3. Run `make test`
4. Open a PR

See the [Filter DSL documentation](https://github.com/edouard-claude/snip/wiki) for available actions.

## Code Changes

1. Fork and create a feature branch
2. Write tests first (TDD)
3. Run `make test && make lint`
4. Commit with [conventional prefixes](https://www.conventionalcommits.org/): `fix:`, `feat:`, `docs:`, `ci:`, `test:`
5. Open a PR against `master`

## Guidelines

- Keep PRs focused — one fix or feature per PR
- All code, comments, and commits in English
- Generated files (`db/`, `gql/generated.go`) should not be edited manually
- Startup performance matters — snip intercepts every shell command

## Questions?

Open a [discussion](https://github.com/edouard-claude/snip/issues) or check existing issues.
