<div align="center">

  <img src="https://github.com/retran/meow/raw/dev/assets/icon_small.png" alt="Meow Logo" width="200">

  <h1>meowg1k</h1>

  <p>Your purr-sonal AI sidekick for coding, writing, and automating anything — right from your terminal.</p>

  <p>
    <a href="https://github.com/retran/meowg1k/stargazers"><img src="https://img.shields.io/github/stars/retran/meowg1k?style=for-the-badge" alt="GitHub stars"></a>
    <a href="https://github.com/retran/meowg1k/network/members"><img src="https://img.shields.io/github/forks/retran/meowg1k?style=for-the-badge" alt="GitHub forks"></a>
    <a href="https://github.com/retran/meowg1k/releases/latest"><img src="https://img.shields.io/github/v/release/retran/meowg1k?style=for-the-badge" alt="Latest Release"></a>
  </p>

  <p>
    <a href="https://github.com/retran/meowg1k/actions/workflows/release.yml"><img src="https://img.shields.io/github/actions/workflow/status/retran/meowg1k/release.yml?branch=dev&style=for-the-badge" alt="Build Status"></a>
    <a href="https://goreportcard.com/report/github.com/retran/meowg1k"><img src="https://goreportcard.com/badge/github.com/retran/meowg1k?style=for-the-badge" alt="Go Report Card"></a>
    <a href="https://github.com/retran/meowg1k/blob/main/go.mod"><img src="https://img.shields.io/github/go-mod/go-version/retran/meowg1k?style=for-the-badge" alt="Go Version"></a>
    <a href="./LICENSE"><img src="https://img.shields.io/github/license/retran/meowg1k?style=for-the-badge" alt="License"></a>
  </p>

</div>

---

`meowg1k` is a command-line interface that brings the power of modern LLMs (Large Language Models) into your terminal. Unlike interactive assistants (like GitHub Copilot or ChatGPT), `meowg1k` is designed for automation and scripting. It's a Unix-philosophy tool that predictably transforms code into an AI-enhanced result.

![meowg1k demo gif](https://user-images.githubusercontent.com/username/image_id.gif)

---

## Who is this for?

`meowg1k` is perfect for:

- **Any developer** who loves the power of the command line and wants to integrate AI into their existing shell workflows.
- **DevOps & Platform Engineers** who want to automate PR descriptions and code analysis in CI/CD pipelines.
- **Security Engineers** who want to run automated code checks using local, private models.

---

## Key Features

- **Built for Automation, Not Conversation:** Perfect for CI/CD, Git hooks, and batch processing.
- **Multi-Provider Support:** Works with Gemini, OpenAI, Anthropic, OpenRouter, and more. Switch between them at any time.
- **Local-First:** Operates with local LLMs (via `llama.cpp`) for complete privacy and offline access.
- **Cost Control:** Built-in token and request rate limiting for predictable spending.
- **Configuration as Code:** Manage all behavior through version-controlled `.yaml` files.
- **Zero Dependencies:** Shipped as a single, native binary. Fast and lightweight.

---

## Quick Start

### 1. Installation

```bash
# Using Go (requires Go 1.25.1+)
go install https://github.com/retran/meowg1k@latest

# Using Homebrew (macOS/Linux)
brew install retran/homebrew-meow-tap/meow
````

> 👉 For other installation methods (Scoop, .deb, .rpm), see the [**full installation guide**](./docs/01-INSTALLATION.md).

### 2. Set up API Key

Get a free API key from [Google AI Studio](https://aistudio.google.com/app/apikey) and add it to your shell profile (`~/.bashrc`, `~/.zshrc`):

```bash
export MEOW_GEMINI_API_KEY="your-api-key-here"
```

Remember to restart your shell or run `source ~/.bashrc`.

### 3. Try It Out

```bash
# Generate code from a prompt
echo "Create a hello world function in Python" | meow g

# Generate a commit message (after staging files)
git add .
meow commit

# Generate a Pull Request description
meow pr --base main
```

---

## Documentation

Explore the full documentation to master `meowg1k`. For a complete overview, jump to our [**Documentation Index**](./docs/README.md).

- [**Installation Guide**](./docs/01-INSTALLATION.md)

  *Get `meowg1k` set up on your system.*

- [**Configuration Guide**](./docs/02-CONFIGURATION.md)

  *Learn how to configure profiles, providers, rate limits, and rules.*

- [**Command Reference**](./docs/03-COMMAND-REFERENCE.md)

  *A detailed reference for all commands and their flags.*

- [**Examples & Recipes**](./docs/04-EXAMPLES.md)

  *Practical examples for solving real-world problems.*

- [**Integrations Guide**](./docs/05-INTEGRATIONS.md)

  *Automate your workflow with Git hooks and CI/CD pipelines.*

- [**Core Principles**](./docs.md/06-PRINCIPLES.md)

  *Understand the philosophy and vision behind the project.*

- [**FAQ**](./docs/07-FAQ.md)

  *Find answers to common questions.*

- [**Troubleshooting Guide**](./docs/08-TROUBLESHOOTING.md)

  *Solve common installation and configuration issues.*

---

## Contributing

We welcome all contributions! This project thrives on community input. Before you get started, please read our [**Contributing Guidelines**](./CONTRIBUTING.md).

All participants are expected to uphold our [**Code of Conduct**](./CODE_OF_CONDUCT.md).

---

## Project Roadmap

Interested in the future of `meowg1k`? Check out our [**Project Roadmap**](./ROADMAP.md) to see what features are planned and where you can help.

---

## Security Policy

Security is a top priority. If you believe you have found a security vulnerability, please follow the responsible disclosure procedure outlined in our [**Security Policy**](./SECURITY.md).

---

## License

This project is licensed under the [**Apache License 2.0**](./LICENSE).

---

<div align="center">

<b>Happy coding with `project meow`! 🐱</b>

<p>Made with ❤️ by Andrew Vasilyev and feline assistants Sonya Blade, Mila, and Marcus Fenix.</p>

[Report Bug](https://github.com/retran/meowg1k/issues) · [Request Feature](https://github.com/retran/meowg1k/issues) · [Contribute](https://github.com/retran/meowg1k/pulls)

</div>
