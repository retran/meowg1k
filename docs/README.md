# Documentation

Welcome to the meowg1k documentation! This directory contains comprehensive guides and API references for developers and users.

## Directory Structure

```
docs/
├── api/                    # API References
│   └── API_REFERENCE.md   # Complete Starlark API documentation
│
└── guides/                # Development Guides
    ├── agentic-system.md  # Agentic workflows and autonomous agents
    ├── architecture.md    # Hexagonal architecture details
    ├── go-conventions.md  # Go coding standards and best practices
    ├── starlark-system.md # Starlark extension system guide
    ├── testing-standards.md # Testing patterns and best practices
    └── ui-patterns.md     # Terminal UI component patterns
```

## Quick Links

### For Users
- **[API Reference](api/API_REFERENCE.md)** - Complete Starlark API with examples
- **[Starlark System Guide](guides/starlark-system.md)** - Learn how to extend meowg1k with custom commands

### For Contributors
- **[Architecture Guide](guides/architecture.md)** - Understand the hexagonal architecture
- **[Go Conventions](guides/go-conventions.md)** - Coding standards for Go development
- **[Testing Standards](guides/testing-standards.md)** - Testing patterns and coverage requirements
- **[UI Patterns](guides/ui-patterns.md)** - Bubble Tea component guidelines

### For Advanced Users
- **[Agentic System](guides/agentic-system.md)** - Build autonomous agent workflows

## Getting Started

1. **Installation** - See main [README](../README.md) for installation instructions
2. **Configuration** - Learn about `.meowg1k/init.star` in the [Starlark System Guide](guides/starlark-system.md)
3. **API Reference** - Explore available modules in the [API Reference](api/API_REFERENCE.md)
4. **Examples** - Check `.meowg1k/commands/` for example command implementations

## Contributing

Please read our [Contributing Guide](../CONTRIBUTING.md) before submitting changes to the documentation.

## Documentation Maintenance

When updating documentation:
- Keep API_REFERENCE.md synchronized with code changes
- Update guides when introducing new patterns or architectural changes
- Add examples to illustrate complex concepts
- Maintain the OpenCode configuration (AGENTS.md, opencode.json) in sync with project structure
