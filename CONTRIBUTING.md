# Contributing to meowg1k

First off, thank you for considering contributing to `meowg1k`! We welcome contributions from everyone. Whether you're reporting a bug, proposing a feature, or writing code, your input is valuable.

This document provides guidelines for contributing to the project.

## Code of Conduct

By participating in this project, you are expected to uphold our [Code of Conduct](./CODE_OF_CONDUCT.md). Please report any unacceptable behavior.

## How Can I Contribute?

There are many ways to contribute to the project:

- **Reporting Bugs:** If you find a bug, please [open an issue](https://github.com/retran/meowg1k/issues/new?template=bug_report.md) and provide as much detail as possible.
- **Suggesting Enhancements:** Have an idea for a new feature or an improvement? [Open a feature request](https://github.com/retran/meowg1k/issues/new?template=feature_request.md).
- **Improving Documentation:** If you find a typo or think a section could be clearer, feel free to open an issue or a pull request.
- **Pull Requests:** If you're ready to contribute code or documentation, we'd love to review your work!

## Development Workflow

This project uses **Go Task** to automate common development tasks. All commands mentioned below should be run from the root of the repository.

### 1. Prerequisites

- **Go 1.25.2** or newer
- **Git**
- **Go Task**: See the [official installation guide](https://taskfile.dev/installation/).

### 2. Set Up Your Environment

Fork the repository to your own GitHub account, then clone it locally.

```bash
git clone git@github.com:retran/meowg1k.git
cd meowg1k
```

Install dependencies and set up your environment by running:

```bash
task setup
```

### 3. Create a Branch

Create a new branch for your changes. Please use a descriptive name.

```bash
# For a new feature
git checkout -b feature/my-awesome-feature

# For a bug fix
git checkout -b fix/resolve-that-bug
```

### 4. Verifying Your Changes Locally

Before committing, use `Taskfile` commands to ensure your code meets project standards.

- **`task fmt`**: Formats all Go code.
- **`task lint`**: Runs the linter to check for style issues.
- **`task test`**: Runs all tests with race detection and generates a coverage report.
- **`task security`**: Runs security scanners (`gosec` and `govulncheck`).
- **`task check`**: A convenient shortcut that runs `vet`, `lint`, and `security` tasks together.

### 5. (Recommended) Install the Pre-commit Hook

To automate checks before every commit, install the Git hook:

```bash
task git:install-hook
```

This hook will automatically run `task git:pre-commit`, which formats, lints, and tests your code. This helps catch errors early and ensures your contributions pass CI.

### 6. Commit Your Changes

We use [Conventional Commits](https://www.conventionalcommits.org/) for our commit messages. This helps us automate changelogs and releases.

Your commit message should be structured like this:

```
<type>(<scope>): <subject>
```

**Examples:**

- `feat(provider): add support for Cohere API`
- `fix(config): handle empty profile gracefully`
- `docs(readme): add example for local models`

> #### 💡 Pro Tip: Use `meowg1k` to write your commit messages!
>
> This is the perfect way to ensure high-quality, consistent commit messages. After staging your files, simply run:
>
> ```bash
> # Stage your changes first
> git add .
>
> # Let meowg1k generate the commit message
> meow commit
> ```
>
> For more complex changes, provide your high-level goal to the model using the `--intent` (or `-i`) flag. This gives the AI crucial context to write a perfect message.
>
> ```bash
> meow commit -i "Refactor database logic to use a connection pool"
> ```

### 7. Submit a Pull Request

Push your branch to your fork and [open a pull request](https://github.com/retran/meowg1k/pulls) against the `dev` branch of the `retran/meowg1k` repository.

- Provide a clear title and description for your PR.
- If it resolves an existing issue, link it using keywords like `Closes #123` or `Fixes #123`.
- Ensure all CI checks have passed.

> #### 💡 Pro Tip: Generate your PR descriptions with `meowg1k`!
>
> Save time and create detailed, structured descriptions for your Pull Requests.
>
> Before creating the PR in the GitHub UI, run the following in your terminal:
>
> ```bash
> # Generate a description for a PR targeting the dev branch
> meow pullrequest --base dev
> ```
>
> Just like with commits, you can significantly improve the result by specifying the main goal of the PR with the `--intent` flag:
>
> ```bash
> meow pullrequest -b dev -i "Implement new user authentication feature using JWT"
> ```
>
> Copy the generated output to use in your Pull Request.

## Continuous Integration (CI)

All pull requests are automatically checked by our CI pipeline on GitHub Actions. A PR must pass all checks before it can be merged. The CI pipeline includes the following jobs:

- **Lint:** Ensures code style and formatting are correct using `golangci-lint`.
- **Security:** Scans for potential vulnerabilities using `gosec` and `govulncheck`.
- **Test and Coverage:** Runs all unit tests, checks for race conditions, and enforces a minimum test coverage threshold of **75%**. A comment with the coverage report will be posted on your PR.

Following the local verification steps and using the pre-commit hook will greatly increase the chances of your PR passing CI on the first try.
