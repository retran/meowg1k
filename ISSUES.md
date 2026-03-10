# Known Issues

Discovered during code review of `.meowg1k/commands/*.star` and `.meowg1k/lib/*.star`.

Status legend: `[ ]` open · `[x]` fixed

---

## Critical Bugs

### [x] `lib/code_search.star` — wrong `ctx.index.search` call signature
**File:** `.meowg1k/lib/code_search.star:373`  
`code_search_handler` calls `ctx.index.search(query, limit=limit)` passing the raw query string as a positional argument.  
The correct API (as used in `commands/search.star:377`) requires an embedding vector and named parameters:
`ctx.index.search(embedding=..., snapshots=..., top_k=..., min_score=...)`.  
The handler must embed the query first via `ctx.llm.embed()` before searching.  
**Impact:** `code_search` tool always fails or returns garbage when used by `code.star` and `new_command.star` agents.

---

## Logic Bugs

### [x] `lib/planning.star` — `execute_plan` tools param is string not list
**File:** `.meowg1k/lib/planning.star:897`  
The `tools` param is defined with `default="[]"` (a JSON string). `getattr(ctx, "tools", [])` returns the string `"[]"` not
a list when no tools are supplied. This string is then passed directly to `ctx.llm.agent_turn(tools=tools_list)`.  
Fix: parse the tools param with `ctx.json.parse()` when it is a string.

### [x] `lib/planning.star` — `decompose_task` ignores `max_depth` param
**File:** `.meowg1k/lib/planning.star:929–947`  
`decompose_task_handler` reads `ctx.task` and calls the LLM but never reads `ctx.max_depth`.  
The `max_depth` param is defined and documented but silently ignored.  
Fix: include `max_depth` in the prompt to the LLM.

### [x] `lib/planning.star` — `create_plan` JSON parse fails on markdown-fenced LLM output
**File:** `.meowg1k/lib/planning.star:891`  
`ctx.json.decode(result)` will hard-fail if the LLM wraps its output in markdown fences (` ```json\n...\n``` `).
The `_PLANNER_SYSTEM` prompt says "no markdown fences" but LLMs sometimes ignore this.  
Fix: strip markdown fences from `result` before parsing.

### [x] `commands/pr.star` — missing `use_session=False` on final LLM call
**File:** `.meowg1k/commands/pr.star:272`  
`ctx.llm.chat(...)` in `generate_pr_description` does not pass `use_session=False`.
`commit.star` does pass it for the equivalent call, preventing the generated description from polluting session history.  
Fix: add `use_session=False` to the `ctx.llm.chat` call.

### [x] `commands/extract.star` — double output (both `ctx.ui.markdown` and `ctx.output.writeline`)
**File:** `.meowg1k/commands/extract.star:134–135`  
`ctx.ui.markdown(output)` renders to the TUI and `ctx.output.writeline(output)` writes to persistent output.
All other commands use only `ctx.output.writeline` for final output (the TUI rendering is handled separately by stream handlers).
This causes the formatted output to appear twice in interactive sessions.  
Fix: remove the `ctx.ui.markdown(output)` call.

### [x] `commands/extract.star` — LLM result assumed to be dict without JSON parsing
**File:** `.meowg1k/commands/extract.star:81`  
The comment says "Result is already a dict when `response_format='json_object'`" but this depends on the
provider/runtime. If the runtime returns a JSON string (not a pre-parsed dict), all subsequent `.get()` calls
will fail.  
Fix: unconditionally parse the result via `ctx.json.parse(result)` to ensure `data` is always a dict.

---

## Documentation Bugs

### [x] `lib/memory.star` — docstring examples use wrong JSON API names
**File:** `.meowg1k/lib/memory.star` (multiple lines in docstring)  
Several examples use `ctx.json.encode(...)` / `ctx.json.decode(...)` but the actual API is
`ctx.json.stringify(...)` / `ctx.json.parse(...)`. Users copying these examples will get runtime errors.  
Fix: update all occurrences in the docstring.

### [x] `lib/code_search.star` — docstring examples use wrong JSON API names
**File:** `.meowg1k/lib/code_search.star` (multiple lines in docstring)  
Same issue: examples use `ctx.json.decode(...)` / `ctx.json.encode(...)` instead of
`ctx.json.parse(...)` / `ctx.json.stringify(...)`.  
Fix: update all occurrences in the docstring.

---

## Minor Issues

### [x] `lib/agent.star` — `compact_threshold` default (60) inconsistent with `compaction.star` default (80)
**File:** `.meowg1k/lib/agent.star:49`, `.meowg1k/lib/compaction.star`  
`run_agent_turn` defaults `compact_threshold=60` but `maybe_compact` defaults to threshold `80`.
Agents using `run_agent_turn` compact 25% more aggressively than the standalone `maybe_compact` default implies.  
Fix: align both defaults to `80` (the documented value).

### [x] `lib/diff.star` — `_MULTI_FILE_ANALYSIS_THRESHOLD` is too low (5)
**File:** `.meowg1k/lib/diff.star`  
Map-reduce summarization is triggered for diffs touching more than 5 files.
For small changes spread across 6 files this causes unnecessary LLM calls for pre-summarization.
A threshold of `10` is more appropriate.  
Fix: raise `_MULTI_FILE_ANALYSIS_THRESHOLD` from `5` to `10`.
