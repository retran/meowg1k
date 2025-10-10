# Frequently Asked Questions (FAQ)

This document answers common questions about `meowg1k`.

---

## General

### Q: Which provider should I use?

- **For beginners:** Gemini (`gemini-2.5-flash`) offers a generous free tier and is very fast.
- **For quality:** Anthropic Claude (`claude-sonnet-4-5-20250929`) provides excellent, high-quality output.
- **For cost-effectiveness:** OpenRouter gives you access to many free and low-cost models.
- **For privacy:** Use a local `llama.cpp` server for complete data privacy.

### Q: Can I use multiple AI providers at the same time?

Yes. This is a core feature. You can define multiple profiles in your `config.yaml`, each pointing to a different provider or model. Then, you can use different profiles for different tasks or even for different file types. See the [Configuration Guide](./02-CONFIGURATION.md) for examples.

### Q: How much does it cost to use?

The cost depends entirely on the provider and model you choose.

- `meowg1k` itself is free and open-source.
- Many providers like Gemini and OpenRouter have free tiers.
- Using a local `llama.cpp` model is completely free, limited only by your hardware.
- To control costs with paid providers, use the rate limiting and token cap features in your profiles.

---

## Configuration

### Q: Where should I put my configuration file?

`meowg1k` looks for `config.yaml` in this order of precedence:

1. **Explicit Path:** A path passed via `--config /path/to/config.yaml`. This overrides everything.
2. **Project Config:** `./.meowg1k/config.yaml`. This is the recommended way to share configuration with your team by committing it to your repository.
3. **User Config:** `~/.config/meowg1k/config.yaml`. Use this for your personal defaults.

### Q: How do I share a configuration with my team?

Create a file at `.meowg1k/config.yaml` in the root of your project and commit it to your Git repository. Team members can still override specific settings locally in their user-level config if needed.

### Q: What happens if a profile isn't found in the config?

If you reference a profile that doesn't exist in your configuration file, `meowg1k` will immediately fail with a clear error message indicating that the requested profile cannot be found. This is intentional behavior to prevent unexpected fallbacks that could lead to unintended API calls or cost.

However, `meowg1k` will fall back to smart defaults for the chosen provider when a profile exists but certain optional fields are missing. For example, if you specify `provider: "gemini"` but don't provide a model, it will default to `gemini-2.5-flash`.

---

## Usage

### Q: Can I pipe content from stdin AND use a user prompt flag?

Yes. This is a primary workflow. The content from stdin provides the **context**, and the `-u, --user-prompt` flag provides the **instruction**.

```bash
cat file.py | meow g -u "Add type hints to this Python code"
```

### Q: How can I debug what is being sent to the AI?

While comprehensive debug logging is planned for a future release, you can currently use several approaches to understand what's being sent:

1. **Deterministic behavior:** The tool's logic is deterministic. Given the same input and configuration, it will always generate the exact same request.
2. **Isolate workflows:** Use a specific test config file (`--config test.yaml`) with known settings to isolate and test specific behaviors.
3. **Provider dashboards:** Most AI providers (OpenAI, Anthropic, Gemini) offer request logs and usage dashboards where you can see the exact prompts and tokens sent.
4. **Small test cases:** Create minimal test inputs and verify the output to build confidence in what's being processed.
5. **Local models:** For complete transparency, use a local `llama.cpp` server where you can enable verbose logging on the server side to see all requests.

### Q: What happens if I hit my provider's rate limits?

You can configure rate limits directly in your `meowg1k` model definition to prevent this. If you do hit a provider limit, the tool will receive an error. By setting `requestsPerMinute` in your config, `meowg1k` will automatically throttle itself to stay under the limit you define.

### Q: Can I use this in CI/CD?

Absolutely. This is a core use case. Use the `--silent` flag to get clean output for scripting, and set your API keys as environment variables in your CI/CD platform's secrets management system.

### Q: Why does `meow pr` require the `--base` flag?

The command needs to know which branch to compare against to generate the list of changes. Common examples are `--base main` or `--base dev`.

While automatic detection of the target branch is a feature under consideration, it's currently explicit by design for several reasons:

- Different projects use different branching strategies (GitFlow, trunk-based, feature branches)
- The default branch might not always be the intended merge target
- Explicit flags prevent accidental comparisons against the wrong branch

You can simplify your workflow by creating shell aliases for common patterns:

```bash
# Add to your ~/.bashrc or ~/.zshrc
alias mpr='meow pr --base main'
alias mprd='meow pr --base dev'
```

Or use git's default branch detection in a script:

```bash
# Get the repository's default branch
DEFAULT_BRANCH=$(git symbolic-ref refs/remotes/origin/HEAD | sed 's@^refs/remotes/origin/@@')
meow pr --base "$DEFAULT_BRANCH"
```

---

## Security & Privacy

### Q: Can I use meowg1k completely offline?

Yes. Configure a profile to use the `llama` provider and point it to a local `llama.cpp` server running on your machine. In this setup, no internet connection is required and no data ever leaves your computer.

### Q: Is my code sent to third parties?

Only if you configure `meowg1k` to use a cloud-based provider like Gemini, OpenAI, or Anthropic. If you use a local model, your code is processed entirely on your machine. You are always in control.

### Q: How are my API keys stored?

They are not. `meowg1k` never writes your API keys to disk. It reads them from environment variables at runtime and holds them in memory only for the duration of a request.

### Q: Is it safe to use in a repository with sensitive or proprietary code?

Yes, provided you use a local model. For maximum security, the recommended approach for sensitive codebases is to use the `llama` provider pointed at a self-hosted LLM.
