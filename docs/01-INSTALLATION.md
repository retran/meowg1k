# Installation Guide

This guide provides instructions for installing `meowg1k` on various operating systems.

---

## Prerequisites

Before you begin, ensure you have the following:

- **Go**: Version 1.25.1 or newer is required if you plan to install using the `go install` method.
- **API Key**: You will need an API key from at least one supported LLM provider (e.g., Gemini, OpenAI, Anthropic) to use the tool with cloud-based models.
- **Internet Connection**: Required for downloading the tool and for using cloud-based models.

---

## Installation Methods

Choose the method that best suits your operating system and workflow.

### Using Go

This is the recommended method for users who have a Go development environment set up.

```bash
go install [github.com/retran/meowg1k@latest](https://github.com/retran/meowg1k@latest)
```

This will download, compile, and install the `meow` binary into your `GOBIN` directory.

### Using Homebrew (macOS)

For macOS users, the easiest way to install and manage `meowg1k` is via Homebrew.

```bash
brew install retran/homebrew-meow-tap/meow
```

### Using Scoop (Windows)

For Windows users, `meowg1k` can be installed via the Scoop package manager.

```powershell
# First, add the bucket
scoop bucket add meow [https://github.com/retran/scoop-meow-bucket.git](https://github.com/retran/scoop-meow-bucket.git)

# Then, install the package
scoop install meow
```

### From Package Files (.deb / .rpm)

For Debian-based and Red Hat-based Linux distributions, you can download a `.deb` or `.rpm` package directly from the [Releases page](https://github.com/retran/meowg1k/releases).

#### For Debian / Ubuntu / Linux Mint (.deb)

Download the appropriate `.deb` file (`amd64` or `arm64`) and install it using `dpkg`:

```bash
sudo dpkg -i meow_<version>_amd64.deb
```

#### For Fedora / CentOS / RHEL (.rpm)

Download the appropriate `.rpm` file (`x86_64` or `aarch64`) and install it using `rpm`:

```bash
sudo rpm -i meow-<version>-1.x86_64.rpm
```

---

## Verifying the Installation

After installing, verify that `meowg1k` is correctly set up by running the `version` command:

```bash
meow version
```

You should see output similar to this, which confirms the tool is in your `PATH` and executable:

```
meow version 85f0b68-dirty
Build Date: 2025-10-03_21:15:50
Git Commit: 85f0b68
```

> **Note:** The version string may look different depending on how you installed it. A suffix like `-dirty` indicates a development build from a local repository with uncommitted changes. Official releases will show a clean version number (e.g., `v1.2.4`).

If you see a "command not found" error, ensure that the installation directory (e.g., `$GOPATH/bin` for Go installs) is included in your shell's `PATH` environment variable.

---

## Next Steps

Once `meowg1k` is installed, the next step is to [configure your API keys and profiles](./02-CONFIGURATION.md).
