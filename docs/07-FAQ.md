# Frequently Asked Questions (FAQ)

This document answers common questions about `meowg1k`.

---

## General

### Q: Which provider should I use?

- **For beginners:** Gemini (`gemini-2.5-flash`) offers a generous free tier and is very fast.
- **For quality:** Anthropic Claude (`claude-4-5-sonnet`) provides excellent, high-quality output.
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

TODO it should fail immediatly

`meowg1k` will fall back to smart defaults for the chosen provider. For example, if you specify `provider: "gemini"` but don't provide a model, it will default to `gemini-1.5-flash-latest`.

---

## Usage

### Q: Can I pipe content from stdin AND use a user prompt flag?

Yes. This is a primary workflow. The content from stdin provides the **context**, and the `-u, --user-prompt` flag provides the **instruction**.

```bash
cat file.py | meow g -u "Add type hints to this Python code"
```

### Q: How can I debug what is being sent to the AI?

TODO we need to implement proper logging

The tool's logic is deterministic. Given the same input and configuration, it will always generate the exact same request. While there is no `--verbose` debug log yet, you can isolate a workflow by using a specific config file (`--config test.yaml`) and providing input directly to see the result.

### Q: What happens if I hit my provider's rate limits?

You can configure rate limits directly in your `meowg1k` profile to prevent this. If you do hit a provider limit, the tool will receive an error. By setting `requestsPerMinute` in your config, `meowg1k` will automatically throttle itself to stay under the limit you define.

### Q: Can I use this in CI/CD?

Absolutely. This is a core use case. Use the `--silent` flag to get clean output for scripting, and set your API keys as environment variables in your CI/CD platform's secrets management system.

### Q: Why does `meow pr` require the `--base` flag?

The command needs to know which branch to compare against to generate the list of changes. Common examples are `--base main` or `--base dev`.

TODO add example with autodetection by git

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
