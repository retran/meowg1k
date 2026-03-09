# Documentation

Two folders, two purposes:

```text
docs/
├── api/          # Starlark API - what you can call
└── guides/       # How to build meowg1k
```

## Pick your path

**Want to use meowg1k?**

- [API Reference](api/API_REFERENCE.md) - All Starlark functions with examples
- [Starlark Guide](guides/starlark-system.md) - Write custom commands

**Want to contribute?**

- [Architecture](guides/architecture.md) - Why we built it this way
- [Go Conventions](guides/go-conventions.md) - Code style rules
- [Testing](guides/testing-standards.md) - How to test your changes
- [UI Patterns](guides/ui-patterns.md) - Build terminal interfaces

**Building autonomous agents?**

- [Agentic System](guides/agentic-system.md) - Multi-step workflows

## First steps

1. Install meowg1k (see root README.md)
2. Create `.meowg1k/init.star` ([Starlark Guide](guides/starlark-system.md) shows how)
3. Browse the [API](api/API_REFERENCE.md) to see what you can do
4. Copy examples from `.meowg1k/commands/`

## Updating docs

Changed the code? Update the docs:

- **Code changes** → update API_REFERENCE.md
- **New patterns** → update relevant guide
- **Complex features** → add examples

Read [CONTRIBUTING.md](../CONTRIBUTING.md) before you start.
