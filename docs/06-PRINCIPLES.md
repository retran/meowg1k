# Project Philosophy & Core Principles

These are the fundamental, non-negotiable principles that guide the architecture and development of `meowg1k`. Every future decision must adhere to these rules.

---

## 1. A Composable Engine, Not an Application

This principle defines the tool's role in the software ecosystem. It is engineered to be a reliable component — a building block — that can be plugged into larger, automated systems. Its primary purpose is to be called programmatically by other tools, such as shell scripts, CI/CD runners, or custom automation pipelines.

This is the opposite of a monolithic, interactive application that a user operates manually. The design prioritizes scriptability, predictable I/O (input/output), and the ability to function as a dependable part of a larger, unattended workflow.

## 2. Task Execution, Not a Conversation

This principle defines the tool's interaction model. It is designed to execute discrete, well-defined tasks that have a clear beginning and end. It is fundamentally not a chatbot or a conversational agent. Its mode of operation is transactional (`Input → Process → Output`), not a continuous, back-and-forth dialogue.

This design choice is critical for ensuring predictability, testability, and suitability for automation. By focusing on atomic tasks, the tool remains a reliable and deterministic engineering component.

## 3. Native Performance. Zero Dependencies

This principle dictates that the tool must be fast, efficient, and easy to deploy anywhere. It is delivered as a single, self-contained native binary. This is a deliberate architectural choice to avoid the overhead and complexity of external runtimes like the JVM, Python, or Node.js.

The direct consequence of this is frictionless deployment and high performance. The tool starts instantly, uses minimal system resources, and can be run in any environment—from a developer's laptop to a stripped-down Docker container—without any installation or configuration of dependencies.

## 4. Radical Independence. No Lock-in

The system is architected to be fundamentally decoupled from any specific vendor, platform, or API. This ensures the user is always in control and is never locked into a single company's ecosystem.

This means that switching between different AI providers (e.g., from a commercial cloud service to a self-hosted local model) is a simple configuration change, not a complex code refactoring. The tool is designed to be a neutral and adaptable client, preserving the user's freedom of choice.

## 5. Local-First Architecture

This principle mandates that the tool's core functionality must be able to operate without a mandatory internet connection. The ability to work offline is a primary design requirement, not an optional feature. This ensures the tool is reliable even with network instability.

More importantly, this architecture guarantees data privacy and user control. Sensitive information, such as proprietary source code, does not need to leave the user's machine unless they explicitly configure the tool to use a cloud-based service.

## 6. Configuration is Code

All aspects of the tool's behavior are defined in structured, version-controllable, plain-text files. The system is built on a hierarchical model, which allows a general, base configuration to be layered with more specific settings for a particular project or environment.

This approach ensures that every workflow is transparent, reproducible, and auditable. An entire setup can be committed to a Git repository, reviewed by team members, and shared, eliminating any "it works on my machine" issues.

## 7. Process Predictability and Audibility

By its nature, the output from an LLM is stochastic (non-deterministic). This principle defines how the tool behaves reliably in that context. The tool's own logic, up to the point of calling an LLM, is strictly deterministic. Given the same input and configuration, it will always generate an identical, byte-for-byte request to the model.

Any randomness in the final result originates exclusively from the AI model's own sampling process. This randomness is, in turn, explicitly controlled by the user through configuration parameters like `temperature`. This design ensures that the tool's process is completely transparent, debuggable, and auditable.

## 8. Intelligent Context, Not Raw Input

The primary value of this tool is not to simply pass text to an AI, but to intelligently prepare the input for the best possible result. The project is guided by the philosophy of automatically enriching a user's prompt with relevant, discoverable context.

This means the tool is committed to evolving its capabilities to analyze source code and its surrounding environment. The long-term vision is to achieve a deep, contextual understanding of the entire workspace by analyzing code statically, leveraging Git history, and employing Retrieval-Augmented Generation on a local code index.

## 9. Security by Design

Security is a foundational requirement, not an add-on. The system is architected under a "zero trust" model where it is designed never to store or persist user secrets, such as API keys. Secrets are only held in memory for the duration of a request and are then discarded.

This principle also extends to the integrity of the tool itself. All official releases must be cryptographically signed. This allows users to independently verify that the executable they are running is the exact one produced by the official build process and has not been altered or compromised.

## 10. Radically Open

The Simple Rule: Every line of code that runs in this tool, including all its dependencies, must be open source.

This is a strict commitment to total transparency, which is essential for trust and security. It means that not only is the project's own code available for inspection, but its entire software supply chain is also auditable. There are no proprietary, closed-source "black boxes" anywhere in the stack.

## 11. You Control the Economics

**The Simple Rule:** The user must have absolute and transparent control over all operational costs.

This principle ensures that operational expenses are a transparent and manageable parameter of the workflow, not a surprising side effect. The architecture provides this control through two primary mechanisms:

1. **Granular Control via Configuration**

    You have full and explicit control over all factors that influence cost, empowering you to make a deliberate economic trade-off for any given task. This includes:

   - **Model & Provider Selection:** The freedom to choose cost-effective models (including free or local ones) over more expensive, powerful ones.
   - **Token Caps:** The ability to set hard limits on `maxInputTokens` and `maxOutputTokens` to prevent unexpectedly large and costly requests.
   - **Rate Limiting:** The power to define strict budgets through `requestsPerMinute` and `requestsPerDay`, creating a hard ceiling on usage.

2. **Built-in Efficiency via Intelligent Context**

    The tool is designed to be inherently cost-effective. Fulfilling **Principle #8 (Intelligent Context, Not Raw Input)**, its primary function is to engineer the context sent to the AI. It automatically optimizes the prompt to use the minimum number of tokens required for a high-quality result. This is a built-in, automatic cost-reduction mechanism that works on your behalf, reducing waste and maximizing the value of every token.
