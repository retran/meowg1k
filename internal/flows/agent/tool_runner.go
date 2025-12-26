// Copyright © 2025 The meowg1k Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/retran/meowg1k/internal/activities/agentstep"
	"github.com/retran/meowg1k/internal/activities/invokellm"
	queryactivity "github.com/retran/meowg1k/internal/activities/query"
	agentconfig "github.com/retran/meowg1k/internal/core/agent"
	"github.com/retran/meowg1k/internal/domain/profile"
	"github.com/retran/meowg1k/internal/ports"
	"github.com/retran/meowg1k/pkg/executor"
)

// ToolRunner executes tool calls for the agent.
type ToolRunner struct {
	filterService    ports.FilterService
	gitService       ports.GitToolingService
	queryFactory     executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output]
	invokeLLMFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output]
	indexFlowBuilder func() (executor.Flow, error)
	planMemory       *planMemory
	memoryStore      *memoryStore
	workspaceRoot    string
	lastIndexSig     string
	searchDefaults   agentconfig.SearchDefaults
	workspaceDirty   bool
}

type planMemory struct {
	items  []planItem
	nextID int
}

type planItem struct {
	Text      string `json:"text"`
	ID        int    `json:"id"`
	Completed bool   `json:"completed"`
}

type memoryStore struct {
	items  []memoryItem
	nextID int
}

type memoryItem struct {
	Text string `json:"text"`
	ID   int    `json:"id"`
}

// NewToolRunner creates a new tool runner.
func NewToolRunner(
	workspaceRoot string,
	filterService ports.FilterService,
	gitService ports.GitToolingService,
	queryFactory executor.ActivityFactory[*queryactivity.Input, *queryactivity.Output],
	invokeLLMFactory executor.ActivityFactory[*invokellm.Input, *invokellm.Output],
	indexFlowBuilder func() (executor.Flow, error),
	searchDefaults agentconfig.SearchDefaults,
) *ToolRunner {
	return &ToolRunner{
		workspaceRoot:    workspaceRoot,
		filterService:    filterService,
		gitService:       gitService,
		queryFactory:     queryFactory,
		invokeLLMFactory: invokeLLMFactory,
		indexFlowBuilder: indexFlowBuilder,
		searchDefaults:   searchDefaults,
		planMemory:       &planMemory{nextID: 1},
		memoryStore:      &memoryStore{nextID: 1},
	}
}

// ResetPlanMemory clears plan tasks between agent runs.
func (r *ToolRunner) ResetPlanMemory() {
	if r == nil {
		return
	}
	r.planMemory = &planMemory{nextID: 1}
}

// RunTool executes a tool call and returns its result.
func (r *ToolRunner) RunTool(ctx context.Context, execCtx *executor.Context, tool, mode string, params json.RawMessage, resolvedProfile *profile.ResolvedProfile, systemPrompt string) (*agentstep.ToolResult, error) {
	toolCtx := execCtx
	toolLabel := fmt.Sprintf("Tool:%s.%s", strings.ToLower(tool), strings.ToLower(mode))
	if execCtx != nil {
		toolCtx = execCtx.Child(toolLabel)
		toolCtx.SendRunning(fmt.Sprintf("Tool: %s.%s", strings.ToLower(tool), strings.ToLower(mode)))
	}

	var result *agentstep.ToolResult
	var err error
	switch strings.ToLower(tool) {
	case "workspace":
		result, err = r.runWorkspaceTool(ctx, toolCtx, mode, params)
	case "search":
		result, err = r.runSearchTool(ctx, toolCtx, mode, params)
	case "git":
		result, err = r.runGitTool(ctx, toolCtx, mode, params)
	case "summarize":
		result, err = r.runSummarizeTool(ctx, toolCtx, mode, params, resolvedProfile, systemPrompt)
	case "plan":
		result, err = r.runPlanTool(mode, params)
	case "memory":
		result, err = r.runMemoryTool(mode, params)
	case "command":
		result, err = r.runCommandTool(ctx, toolCtx, mode, params)
	case "patch":
		result, err = r.runPatchTool(ctx, toolCtx, mode, params)
	default:
		err = fmt.Errorf("unsupported tool %q", tool)
	}

	if err != nil {
		if toolCtx != nil {
			toolCtx.SendFailed(err, fmt.Sprintf("Tool failed: %s.%s", strings.ToLower(tool), strings.ToLower(mode)))
		}
		return nil, err
	}

	if toolCtx != nil {
		toolCtx.SendCompleted(fmt.Sprintf("%s.%s", strings.ToLower(tool), strings.ToLower(mode)))
	}

	return result, nil
}

type workspaceListParams struct {
	IncludeFiles *bool  `json:"include_files"`
	IncludeDirs  *bool  `json:"include_dirs"`
	Path         string `json:"path"`
	Depth        int    `json:"depth"`
}

type workspaceReadParams struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type workspaceWriteParams struct {
	Create    *bool  `json:"create"`
	Overwrite *bool  `json:"overwrite"`
	Path      string `json:"path"`
	Content   string `json:"content"`
}

type workspaceReplaceParams struct {
	RequireMatch *bool  `json:"require_match"`
	Path         string `json:"path"`
	OldText      string `json:"old_text"`
	NewText      string `json:"new_text"`
	Occurrence   string `json:"occurrence"`
}

type workspaceDeleteParams struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

type workspaceMkdirParams struct {
	Parents *bool  `json:"parents"`
	Path    string `json:"path"`
}

type workspaceStatParams struct {
	Path string `json:"path"`
}

const (
	modeList = "list"
)

func (r *ToolRunner) runWorkspaceTool(ctx context.Context, _ *executor.Context, mode string, params json.RawMessage) (*agentstep.ToolResult, error) {
	switch strings.ToLower(mode) {
	case modeList:
		return r.handleWorkspaceList(ctx, params)
	case "read":
		return r.handleWorkspaceRead(params)
	case "write":
		return r.handleWorkspaceWrite(params)
	case "replace":
		return r.handleWorkspaceReplace(params)
	case "delete":
		return r.handleWorkspaceDelete(params)
	case "mkdir":
		return r.handleWorkspaceMkdir(params)
	case "stat":
		return r.handleWorkspaceStat(params)
	case "exists":
		return r.handleWorkspaceExists(params)
	default:
		return nil, fmt.Errorf("unsupported workspace mode %q", mode)
	}
}

func (r *ToolRunner) handleWorkspaceList(ctx context.Context, params json.RawMessage) (*agentstep.ToolResult, error) {
	var input workspaceListParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse workspace list params: %w", err)
	}
	result, err := r.workspaceList(ctx, input)
	if err != nil {
		return nil, err
	}
	return &agentstep.ToolResult{Tool: "workspace", Mode: modeList, Data: result}, nil
}

func (r *ToolRunner) handleWorkspaceRead(params json.RawMessage) (*agentstep.ToolResult, error) {
	var input workspaceReadParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse workspace read params: %w", err)
	}
	result, err := r.workspaceRead(input)
	if err != nil {
		return nil, err
	}
	return &agentstep.ToolResult{Tool: "workspace", Mode: "read", Data: result}, nil
}

func (r *ToolRunner) handleWorkspaceWrite(params json.RawMessage) (*agentstep.ToolResult, error) {
	var input workspaceWriteParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse workspace write params: %w", err)
	}
	result, err := r.workspaceWrite(input)
	if err != nil {
		return nil, err
	}
	r.workspaceDirty = true
	return &agentstep.ToolResult{Tool: "workspace", Mode: "write", Data: result}, nil
}

func (r *ToolRunner) handleWorkspaceReplace(params json.RawMessage) (*agentstep.ToolResult, error) {
	var input workspaceReplaceParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse workspace replace params: %w", err)
	}
	result, err := r.workspaceReplace(input)
	if err != nil {
		return nil, err
	}
	r.workspaceDirty = true
	return &agentstep.ToolResult{Tool: "workspace", Mode: "replace", Data: result}, nil
}

func (r *ToolRunner) handleWorkspaceDelete(params json.RawMessage) (*agentstep.ToolResult, error) {
	var input workspaceDeleteParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse workspace delete params: %w", err)
	}
	result, err := r.workspaceDelete(input)
	if err != nil {
		return nil, err
	}
	r.workspaceDirty = true
	return &agentstep.ToolResult{Tool: "workspace", Mode: "delete", Data: result}, nil
}

func (r *ToolRunner) handleWorkspaceMkdir(params json.RawMessage) (*agentstep.ToolResult, error) {
	var input workspaceMkdirParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse workspace mkdir params: %w", err)
	}
	result, err := r.workspaceMkdir(input)
	if err != nil {
		return nil, err
	}
	r.workspaceDirty = true
	return &agentstep.ToolResult{Tool: "workspace", Mode: "mkdir", Data: result}, nil
}

func (r *ToolRunner) handleWorkspaceStat(params json.RawMessage) (*agentstep.ToolResult, error) {
	var input workspaceStatParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse workspace stat params: %w", err)
	}
	result, err := r.workspaceStat(input)
	if err != nil {
		return nil, err
	}
	return &agentstep.ToolResult{Tool: "workspace", Mode: "stat", Data: result}, nil
}

func (r *ToolRunner) handleWorkspaceExists(params json.RawMessage) (*agentstep.ToolResult, error) {
	var input workspaceStatParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse workspace exists params: %w", err)
	}
	result, err := r.workspaceExists(input)
	if err != nil {
		return nil, err
	}
	return &agentstep.ToolResult{Tool: "workspace", Mode: "exists", Data: result}, nil
}

type listEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

func (r *ToolRunner) workspaceList(ctx context.Context, input workspaceListParams) ([]listEntry, error) {
	options, err := r.resolveListOptions(input)
	if err != nil {
		return nil, err
	}

	entries := make([]listEntry, 0)
	err = filepath.WalkDir(options.basePath, func(path string, d fs.DirEntry, walkErr error) error {
		return r.visitWorkspaceEntry(ctx, options, &entries, path, d, walkErr)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace entries under %s: %w", options.baseRel, err)
	}

	return entries, nil
}

type listOptions struct {
	basePath     string
	baseRel      string
	depth        int
	includeFiles bool
	includeDirs  bool
}

func (r *ToolRunner) resolveListOptions(input workspaceListParams) (listOptions, error) {
	basePath, err := r.resolveWorkspacePath(input.Path)
	if err != nil {
		return listOptions{}, err
	}

	baseRel, err := filepath.Rel(r.workspaceRoot, basePath)
	if err != nil {
		return listOptions{}, fmt.Errorf("failed to compute base relative path: %w", err)
	}

	depth := input.Depth
	if depth <= 0 {
		depth = 1
	}

	includeFiles := true
	if input.IncludeFiles != nil {
		includeFiles = *input.IncludeFiles
	}
	includeDirs := true
	if input.IncludeDirs != nil {
		includeDirs = *input.IncludeDirs
	}

	return listOptions{
		basePath:     basePath,
		baseRel:      baseRel,
		depth:        depth,
		includeFiles: includeFiles,
		includeDirs:  includeDirs,
	}, nil
}

func (r *ToolRunner) visitWorkspaceEntry(ctx context.Context, options listOptions, entries *[]listEntry, path string, d fs.DirEntry, walkErr error) error {
	action, rel, relativeToBase, err := r.classifyWorkspaceEntry(ctx, options, path, d, walkErr)
	if err != nil {
		return err
	}

	switch action {
	case entrySkipDir:
		return filepath.SkipDir
	case entrySkip:
		return nil
	case entryInclude:
		break
	}

	r.appendWorkspaceEntry(entries, rel, relativeToBase, d, options)
	return nil
}

type entryAction int

const (
	entryInclude entryAction = iota
	entrySkip
	entrySkipDir
)

func (r *ToolRunner) classifyWorkspaceEntry(ctx context.Context, options listOptions, path string, d fs.DirEntry, walkErr error) (action entryAction, rel string, relativeToBase string, err error) {
	if walkErr != nil {
		return entrySkip, "", "", walkErr
	}
	if err := ctx.Err(); err != nil {
		return entrySkip, "", "", fmt.Errorf("workspace walk canceled: %w", err)
	}

	rel, err = filepath.Rel(r.workspaceRoot, path)
	if err != nil {
		return entrySkip, "", "", fmt.Errorf("failed to compute relative path: %w", err)
	}

	if r.filterService != nil && r.filterService.IsIgnoredFile(rel) {
		if d.IsDir() {
			return entrySkipDir, "", "", nil
		}
		return entrySkip, "", "", nil
	}

	relativeToBase, err = filepath.Rel(options.basePath, path)
	if err != nil {
		return entrySkip, "", "", fmt.Errorf("failed to compute base-relative path: %w", err)
	}

	if relativeToBase != "." && pathDepth(relativeToBase) > options.depth {
		if d.IsDir() {
			return entrySkipDir, "", "", nil
		}
		return entrySkip, "", "", nil
	}

	return entryInclude, rel, relativeToBase, nil
}

func (r *ToolRunner) appendWorkspaceEntry(entries *[]listEntry, rel string, relativeToBase string, d fs.DirEntry, options listOptions) {
	if d.IsDir() && options.includeDirs && relativeToBase != "." {
		*entries = append(*entries, listEntry{Path: filepath.ToSlash(rel), IsDir: true})
	}
	if !d.IsDir() && options.includeFiles {
		*entries = append(*entries, listEntry{Path: filepath.ToSlash(rel), IsDir: false})
	}
}

func pathDepth(rel string) int {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	depth := 0
	for _, part := range parts {
		if part != "" && part != "." {
			depth++
		}
	}
	return depth
}

type readResult struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

func (r *ToolRunner) workspaceRead(input workspaceReadParams) (*readResult, error) {
	path, err := r.resolveWorkspacePath(input.Path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(r.workspaceRoot, path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve relative path: %w", err)
	}
	if r.filterService != nil && r.filterService.IsIgnoredFile(rel) {
		return nil, fmt.Errorf("path is ignored: %s", rel)
	}

	relPath := filepath.ToSlash(rel)
	if !fs.ValidPath(relPath) {
		return nil, fmt.Errorf("invalid relative path: %s", rel)
	}
	content, err := fs.ReadFile(os.DirFS(r.workspaceRoot), relPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", rel, err)
	}
	lines := strings.Split(string(content), "\n")
	start := input.StartLine
	end := input.EndLine
	if start <= 0 {
		start = 1
	}
	if end <= 0 || end > len(lines) {
		end = len(lines)
	}
	if start > end {
		return nil, fmt.Errorf("start_line cannot be greater than end_line")
	}

	selected := strings.Join(lines[start-1:end], "\n")
	return &readResult{
		Path:      filepath.ToSlash(rel),
		Content:   selected,
		StartLine: start,
		EndLine:   end,
	}, nil
}

type writeResult struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Bytes  int    `json:"bytes"`
}

func (r *ToolRunner) workspaceWrite(input workspaceWriteParams) (*writeResult, error) {
	path, err := r.resolveWorkspacePath(input.Path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(r.workspaceRoot, path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve relative path: %w", err)
	}
	if r.filterService != nil && r.filterService.IsIgnoredFile(rel) {
		return nil, fmt.Errorf("path is ignored: %s", rel)
	}

	create := true
	if input.Create != nil {
		create = *input.Create
	}
	overwrite := true
	if input.Overwrite != nil {
		overwrite = *input.Overwrite
	}

	if !create {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("file does not exist: %s", rel)
		}
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return nil, fmt.Errorf("file exists and overwrite is false: %s", rel)
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create parent directories: %w", err)
	}
	if err := os.WriteFile(path, []byte(input.Content), 0o600); err != nil {
		return nil, fmt.Errorf("failed to write file %s: %w", rel, err)
	}

	return &writeResult{
		Path:   filepath.ToSlash(rel),
		Bytes:  len(input.Content),
		Status: "written",
	}, nil
}

type replaceResult struct {
	Path         string `json:"path"`
	Occurrence   string `json:"occurrence"`
	Matches      int    `json:"matches"`
	Replacements int    `json:"replacements"`
	RequireMatch bool   `json:"require_match"`
}

func (r *ToolRunner) workspaceReplace(input workspaceReplaceParams) (*replaceResult, error) {
	path, rel, content, err := r.loadWorkspaceFile(input.Path)
	if err != nil {
		return nil, err
	}

	occurrence, requireMatch := normalizeReplaceOptions(input)
	matches := strings.Count(string(content), input.OldText)
	if requireMatch && matches == 0 {
		return nil, fmt.Errorf("no matches found in %s", rel)
	}

	updated, replacements, err := applyReplacement(string(content), input, matches, occurrence, rel)
	if err != nil {
		return nil, err
	}

	result := &replaceResult{
		Path:         filepath.ToSlash(rel),
		Matches:      matches,
		Replacements: replacements,
		Occurrence:   occurrence,
		RequireMatch: requireMatch,
	}

	if updated == string(content) {
		return result, nil
	}

	if err := os.WriteFile(path, []byte(updated), 0o600); err != nil {
		return nil, fmt.Errorf("failed to write file %s: %w", rel, err)
	}

	return result, nil
}

func (r *ToolRunner) loadWorkspaceFile(path string) (resolved string, rel string, content []byte, err error) {
	resolved, err = r.resolveWorkspacePath(path)
	if err != nil {
		return "", "", nil, err
	}
	rel, err = filepath.Rel(r.workspaceRoot, resolved)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to resolve relative path: %w", err)
	}
	if r.filterService != nil && r.filterService.IsIgnoredFile(rel) {
		return "", "", nil, fmt.Errorf("path is ignored: %s", rel)
	}

	relPath := filepath.ToSlash(rel)
	if !fs.ValidPath(relPath) {
		return "", "", nil, fmt.Errorf("invalid relative path: %s", rel)
	}
	content, err = fs.ReadFile(os.DirFS(r.workspaceRoot), relPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read file %s: %w", rel, err)
	}
	return resolved, rel, content, nil
}

func normalizeReplaceOptions(input workspaceReplaceParams) (string, bool) {
	occurrence := strings.ToLower(strings.TrimSpace(input.Occurrence))
	if occurrence == "" {
		occurrence = "all"
	}

	requireMatch := true
	if input.RequireMatch != nil {
		requireMatch = *input.RequireMatch
	}
	return occurrence, requireMatch
}

func applyReplacement(content string, input workspaceReplaceParams, matches int, occurrence, rel string) (updated string, replacements int, err error) {
	switch occurrence {
	case "all":
		return strings.ReplaceAll(content, input.OldText, input.NewText), matches, nil
	case "first":
		if matches == 0 {
			return content, 0, nil
		}
		return strings.Replace(content, input.OldText, input.NewText, 1), 1, nil
	case "single":
		if matches != 1 {
			return "", 0, fmt.Errorf("expected exactly one match in %s, found %d", rel, matches)
		}
		return strings.Replace(content, input.OldText, input.NewText, 1), 1, nil
	default:
		return "", 0, fmt.Errorf("unsupported occurrence %q", occurrence)
	}
}

type deleteResult struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

func (r *ToolRunner) workspaceDelete(input workspaceDeleteParams) (*deleteResult, error) {
	path, err := r.resolveWorkspacePath(input.Path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(r.workspaceRoot, path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve relative path: %w", err)
	}
	if r.filterService != nil && r.filterService.IsIgnoredFile(rel) {
		return nil, fmt.Errorf("path is ignored: %s", rel)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat %s: %w", rel, err)
	}

	if info.IsDir() && !input.Recursive {
		return nil, fmt.Errorf("path is a directory; recursive delete is false")
	}

	if info.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			return nil, fmt.Errorf("failed to remove directory %s: %w", rel, err)
		}
	} else {
		if err := os.Remove(path); err != nil {
			return nil, fmt.Errorf("failed to remove file %s: %w", rel, err)
		}
	}

	return &deleteResult{Path: filepath.ToSlash(rel), Status: "deleted"}, nil
}

type mkdirResult struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

func (r *ToolRunner) workspaceMkdir(input workspaceMkdirParams) (*mkdirResult, error) {
	path, err := r.resolveWorkspacePath(input.Path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(r.workspaceRoot, path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve relative path: %w", err)
	}
	if r.filterService != nil && r.filterService.IsIgnoredFile(rel) {
		return nil, fmt.Errorf("path is ignored: %s", rel)
	}

	parents := true
	if input.Parents != nil {
		parents = *input.Parents
	}

	if parents {
		if err := os.MkdirAll(path, 0o750); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", rel, err)
		}
	} else {
		if err := os.Mkdir(path, 0o750); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", rel, err)
		}
	}

	return &mkdirResult{Path: filepath.ToSlash(rel), Status: "created"}, nil
}

type statResult struct {
	ModTime time.Time `json:"mod_time"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	Exists  bool      `json:"exists"`
	IsDir   bool      `json:"is_dir"`
}

func (r *ToolRunner) workspaceStat(input workspaceStatParams) (*statResult, error) {
	path, err := r.resolveWorkspacePath(input.Path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(r.workspaceRoot, path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve relative path: %w", err)
	}
	if r.filterService != nil && r.filterService.IsIgnoredFile(rel) {
		return nil, fmt.Errorf("path is ignored: %s", rel)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat %s: %w", rel, err)
	}

	return &statResult{
		Path:    filepath.ToSlash(rel),
		Exists:  true,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

type existsResult struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

func (r *ToolRunner) workspaceExists(input workspaceStatParams) (*existsResult, error) {
	path, err := r.resolveWorkspacePath(input.Path)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(r.workspaceRoot, path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve relative path: %w", err)
	}
	if r.filterService != nil && r.filterService.IsIgnoredFile(rel) {
		return nil, fmt.Errorf("path is ignored: %s", rel)
	}

	_, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &existsResult{Path: filepath.ToSlash(rel), Exists: false}, nil
		}
		return nil, fmt.Errorf("failed to stat %s: %w", rel, err)
	}

	return &existsResult{Path: filepath.ToSlash(rel), Exists: true}, nil
}

func (r *ToolRunner) resolveWorkspacePath(path string) (string, error) {
	if r.workspaceRoot == "" {
		return "", fmt.Errorf("workspace root is empty")
	}

	cleanRoot := filepath.Clean(r.workspaceRoot)
	target := path
	if strings.TrimSpace(target) == "" {
		target = cleanRoot
	}

	if !filepath.IsAbs(target) {
		target = filepath.Join(cleanRoot, target)
	}
	target = filepath.Clean(target)

	if !strings.HasPrefix(target, cleanRoot+string(filepath.Separator)) && target != cleanRoot {
		return "", fmt.Errorf("path is outside workspace root: %s", path)
	}

	return target, nil
}

type searchParams struct {
	QueryText string `json:"query_text"`
}

type searchResult struct {
	FilePath  string  `json:"file_path"`
	Snippet   string  `json:"snippet"`
	Score     float32 `json:"score"`
	StartLine int     `json:"start_line"`
	EndLine   int     `json:"end_line"`
}

func (r *ToolRunner) runSearchTool(ctx context.Context, execCtx *executor.Context, mode string, params json.RawMessage) (*agentstep.ToolResult, error) {
	if !strings.EqualFold(mode, "embeddings") {
		return nil, fmt.Errorf("unsupported search mode %q", mode)
	}

	var input searchParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse search params: %w", err)
	}
	if strings.TrimSpace(input.QueryText) == "" {
		return nil, fmt.Errorf("query_text is required")
	}

	if err := r.ensureIndex(ctx, execCtx); err != nil {
		return nil, err
	}

	activity := r.queryFactory.NewActivity()
	executorInstance := execCtx.GetExecutor()
	queryInput := &queryactivity.Input{
		QueryText:        input.QueryText,
		SnapshotPriority: r.searchDefaults.Snapshots,
		TopK:             r.searchDefaults.TopK,
		MinScore:         r.searchDefaults.MinScore,
	}
	output, err := executor.ExecuteActivity(ctx, executorInstance, execCtx, "Query", activity, queryInput)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	results := make([]searchResult, 0, len(output.Results))
	for _, result := range output.Results {
		results = append(results, searchResult{
			FilePath:  result.FilePath,
			StartLine: result.StartLine,
			EndLine:   result.EndLine,
			Score:     result.Score,
			Snippet:   result.TextContent,
		})
	}

	return &agentstep.ToolResult{Tool: "search", Mode: "embeddings", Data: results}, nil
}

func (r *ToolRunner) ensureIndex(ctx context.Context, execCtx *executor.Context) error {
	if r.indexFlowBuilder == nil {
		return nil
	}

	sig, err := r.currentIndexSignature()
	if err != nil {
		if r.workspaceDirty {
			return r.runIndexFlow(ctx, execCtx)
		}
		return nil
	}

	if r.workspaceDirty || sig != r.lastIndexSig {
		if err := r.runIndexFlow(ctx, execCtx); err != nil {
			return err
		}
		r.lastIndexSig = sig
		r.workspaceDirty = false
	}
	return nil
}

func (r *ToolRunner) currentIndexSignature() (string, error) {
	if r.gitService == nil {
		return "", fmt.Errorf("git service is nil")
	}
	status, err := r.gitService.Status()
	if err != nil {
		return "", fmt.Errorf("git status failed: %w", err)
	}
	head, err := r.gitService.HeadHash()
	if err != nil {
		return "", fmt.Errorf("git head hash failed: %w", err)
	}
	return fmt.Sprintf("%s|%s", head, status), nil
}

func (r *ToolRunner) runIndexFlow(ctx context.Context, execCtx *executor.Context) error {
	flow, err := r.indexFlowBuilder()
	if err != nil {
		return fmt.Errorf("failed to build index flow: %w", err)
	}
	executorInstance := execCtx.GetExecutor()
	if executorInstance == nil {
		return fmt.Errorf("executor not available")
	}
	if err := executorInstance.ExecuteFlow(ctx, "IndexFlow", flow); err != nil {
		return fmt.Errorf("index flow failed: %w", err)
	}
	return nil
}

type gitDiffParams struct {
	Ref  string `json:"ref"`
	Path string `json:"path"`
}

type gitShowParams struct {
	Ref string `json:"ref"`
}

type gitLogParams struct {
	Path  string `json:"path"`
	Limit int    `json:"limit"`
}

type gitStageParams struct {
	Paths []string `json:"paths"`
}

type gitCommitParams struct {
	Message string `json:"message"`
}

func (r *ToolRunner) runGitTool(_ context.Context, _ *executor.Context, mode string, params json.RawMessage) (*agentstep.ToolResult, error) {
	if r.gitService == nil {
		return nil, fmt.Errorf("git service is nil")
	}

	handlers := map[string]func(json.RawMessage) (*agentstep.ToolResult, error){
		"status":         r.handleGitStatus,
		"diff":           r.handleGitDiff,
		"show":           r.handleGitShow,
		"log":            r.handleGitLog,
		"branch":         r.handleGitBranch,
		"current_branch": r.handleGitCurrentBranch,
		"stage":          r.handleGitStage,
		"commit":         r.handleGitCommit,
	}

	handler, ok := handlers[strings.ToLower(mode)]
	if !ok {
		return nil, fmt.Errorf("unsupported git mode %q", mode)
	}
	return handler(params)
}

func (r *ToolRunner) handleGitStatus(_ json.RawMessage) (*agentstep.ToolResult, error) {
	status, err := r.gitService.Status()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}
	return &agentstep.ToolResult{Tool: "git", Mode: "status", Data: map[string]string{"status": status}}, nil
}

func (r *ToolRunner) handleGitDiff(params json.RawMessage) (*agentstep.ToolResult, error) {
	return handleGitWithParams(params, "diff", "diff", func(input gitDiffParams) (string, error) {
		return r.gitService.Diff(input.Ref, input.Path)
	})
}

func (r *ToolRunner) handleGitShow(params json.RawMessage) (*agentstep.ToolResult, error) {
	var input gitShowParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse git show params: %w", err)
	}
	show, err := r.gitService.Show(input.Ref)
	if err != nil {
		return nil, fmt.Errorf("git show failed: %w", err)
	}
	return &agentstep.ToolResult{Tool: "git", Mode: "show", Data: map[string]string{"show": show}}, nil
}

func (r *ToolRunner) handleGitLog(params json.RawMessage) (*agentstep.ToolResult, error) {
	return handleGitWithParams(params, "log", "log", func(input gitLogParams) (string, error) {
		return r.gitService.Log(input.Limit, input.Path)
	})
}

func (r *ToolRunner) handleGitBranch(_ json.RawMessage) (*agentstep.ToolResult, error) {
	branches, err := r.gitService.Branches()
	if err != nil {
		return nil, fmt.Errorf("git branch failed: %w", err)
	}
	return &agentstep.ToolResult{Tool: "git", Mode: "branch", Data: branches}, nil
}

func (r *ToolRunner) handleGitCurrentBranch(_ json.RawMessage) (*agentstep.ToolResult, error) {
	branch, err := r.gitService.CurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("git current_branch failed: %w", err)
	}
	return &agentstep.ToolResult{Tool: "git", Mode: "current_branch", Data: map[string]string{"branch": branch}}, nil
}

func (r *ToolRunner) handleGitStage(params json.RawMessage) (*agentstep.ToolResult, error) {
	var input gitStageParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse git stage params: %w", err)
	}
	output, err := r.gitService.Stage(input.Paths)
	if err != nil {
		return nil, fmt.Errorf("git stage failed: %w", err)
	}
	return &agentstep.ToolResult{Tool: "git", Mode: "stage", Data: map[string]string{"output": output}}, nil
}

func (r *ToolRunner) handleGitCommit(params json.RawMessage) (*agentstep.ToolResult, error) {
	var input gitCommitParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse git commit params: %w", err)
	}
	output, err := r.gitService.Commit(input.Message)
	if err != nil {
		return nil, fmt.Errorf("git commit failed: %w", err)
	}
	return &agentstep.ToolResult{Tool: "git", Mode: "commit", Data: map[string]string{"output": output}}, nil
}

func handleGitWithParams[T any](params json.RawMessage, mode string, resultKey string, action func(T) (string, error)) (*agentstep.ToolResult, error) {
	var input T
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse git %s params: %w", mode, err)
	}
	output, err := action(input)
	if err != nil {
		return nil, fmt.Errorf("git %s failed: %w", mode, err)
	}
	return &agentstep.ToolResult{Tool: "git", Mode: mode, Data: map[string]string{resultKey: output}}, nil
}

type summarizeTextParams struct {
	Text string `json:"text"`
}

type summarizeFileParams struct {
	Path string `json:"path"`
}

func (r *ToolRunner) runSummarizeTool(ctx context.Context, execCtx *executor.Context, mode string, params json.RawMessage, resolvedProfile *profile.ResolvedProfile, systemPrompt string) (*agentstep.ToolResult, error) {
	handlers := map[string]func(json.RawMessage) (*agentstep.ToolResult, error){
		"text": func(raw json.RawMessage) (*agentstep.ToolResult, error) {
			return r.summarizeText(ctx, execCtx, raw, resolvedProfile, systemPrompt)
		},
		"file": func(raw json.RawMessage) (*agentstep.ToolResult, error) {
			return r.summarizeFile(ctx, execCtx, raw, resolvedProfile, systemPrompt)
		},
		"diff": func(raw json.RawMessage) (*agentstep.ToolResult, error) {
			return r.summarizeDiff(ctx, execCtx, raw, resolvedProfile, systemPrompt)
		},
	}

	handler, ok := handlers[strings.ToLower(mode)]
	if !ok {
		return nil, fmt.Errorf("unsupported summarize mode %q", mode)
	}
	return handler(params)
}

func (r *ToolRunner) summarizeText(ctx context.Context, execCtx *executor.Context, params json.RawMessage, resolvedProfile *profile.ResolvedProfile, systemPrompt string) (*agentstep.ToolResult, error) {
	var input summarizeTextParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse summarize text params: %w", err)
	}
	if strings.TrimSpace(input.Text) == "" {
		return nil, fmt.Errorf("text is required")
	}
	summary, err := r.summarize(ctx, execCtx, input.Text, resolvedProfile, systemPrompt)
	if err != nil {
		return nil, err
	}
	return &agentstep.ToolResult{Tool: "summarize", Mode: "text", Data: map[string]string{"summary": summary}}, nil
}

func (r *ToolRunner) summarizeFile(ctx context.Context, execCtx *executor.Context, params json.RawMessage, resolvedProfile *profile.ResolvedProfile, systemPrompt string) (*agentstep.ToolResult, error) {
	var input summarizeFileParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse summarize file params: %w", err)
	}
	readResult, err := r.workspaceRead(workspaceReadParams{Path: input.Path})
	if err != nil {
		return nil, err
	}
	summary, err := r.summarize(ctx, execCtx, readResult.Content, resolvedProfile, systemPrompt)
	if err != nil {
		return nil, err
	}
	return &agentstep.ToolResult{Tool: "summarize", Mode: "file", Data: map[string]string{"summary": summary}}, nil
}

func (r *ToolRunner) summarizeDiff(ctx context.Context, execCtx *executor.Context, params json.RawMessage, resolvedProfile *profile.ResolvedProfile, systemPrompt string) (*agentstep.ToolResult, error) {
	var input gitDiffParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse summarize diff params: %w", err)
	}
	if r.gitService == nil {
		return nil, fmt.Errorf("git service is nil")
	}
	diff, err := r.gitService.Diff(input.Ref, input.Path)
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	summary, err := r.summarize(ctx, execCtx, diff, resolvedProfile, systemPrompt)
	if err != nil {
		return nil, err
	}
	return &agentstep.ToolResult{Tool: "summarize", Mode: "diff", Data: map[string]string{"summary": summary}}, nil
}

func (r *ToolRunner) summarize(ctx context.Context, execCtx *executor.Context, text string, resolvedProfile *profile.ResolvedProfile, stepPrompt string) (string, error) {
	if r.invokeLLMFactory == nil {
		return "", fmt.Errorf("invoke llm factory is nil")
	}
	executorInstance := execCtx.GetExecutor()
	if executorInstance == nil {
		return "", fmt.Errorf("executor not available")
	}
	if resolvedProfile == nil {
		return "", fmt.Errorf("profile is nil")
	}

	systemPrompt := "You are a summarization assistant. Provide a concise summary of the input."
	if strings.TrimSpace(stepPrompt) != "" {
		systemPrompt = strings.TrimSpace(stepPrompt) + "\n\n" + systemPrompt
	}
	userPrompt := text

	activity := r.invokeLLMFactory.NewActivity()
	input := &invokellm.Input{
		Profile:      resolvedProfile,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
	}
	output, err := executor.ExecuteActivity(ctx, executorInstance, execCtx, "InvokeLLM", activity, input)
	if err != nil {
		return "", fmt.Errorf("summarize failed: %w", err)
	}

	return strings.TrimSpace(output.Content), nil
}

func (r *ToolRunner) runPlanTool(mode string, params json.RawMessage) (*agentstep.ToolResult, error) {
	switch strings.ToLower(mode) {
	case "add":
		var input struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("failed to parse plan add params: %w", err)
		}
		if strings.TrimSpace(input.Text) == "" {
			return nil, fmt.Errorf("text is required")
		}
		item := planItem{
			ID:        r.planMemory.nextID,
			Text:      input.Text,
			Completed: false,
		}
		r.planMemory.nextID++
		r.planMemory.items = append(r.planMemory.items, item)
		return &agentstep.ToolResult{Tool: "plan", Mode: "add", Data: item}, nil
	case "complete":
		var input struct {
			TaskID int `json:"task_id"`
		}
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("failed to parse plan complete params: %w", err)
		}
		for i, item := range r.planMemory.items {
			if item.ID == input.TaskID {
				r.planMemory.items[i].Completed = true
				return &agentstep.ToolResult{Tool: "plan", Mode: "complete", Data: r.planMemory.items[i]}, nil
			}
		}
		return nil, fmt.Errorf("plan item not found: %d", input.TaskID)
	case modeList:
		return &agentstep.ToolResult{Tool: "plan", Mode: modeList, Data: r.planMemory.items}, nil
	default:
		return nil, fmt.Errorf("unsupported plan mode %q", mode)
	}
}

func (r *ToolRunner) runMemoryTool(mode string, params json.RawMessage) (*agentstep.ToolResult, error) {
	switch strings.ToLower(mode) {
	case "add":
		var input struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, fmt.Errorf("failed to parse memory add params: %w", err)
		}
		if strings.TrimSpace(input.Text) == "" {
			return nil, fmt.Errorf("text is required")
		}
		item := memoryItem{
			ID:   r.memoryStore.nextID,
			Text: input.Text,
		}
		r.memoryStore.nextID++
		r.memoryStore.items = append(r.memoryStore.items, item)
		return &agentstep.ToolResult{Tool: "memory", Mode: "add", Data: item}, nil
	case modeList:
		return &agentstep.ToolResult{Tool: "memory", Mode: modeList, Data: r.memoryStore.items}, nil
	default:
		return nil, fmt.Errorf("unsupported memory mode %q", mode)
	}
}

type commandParams struct {
	Command string `json:"command"`
	Workdir string `json:"workdir"`
	Timeout string `json:"timeout"`
}

func (r *ToolRunner) runCommandTool(ctx context.Context, _ *executor.Context, mode string, params json.RawMessage) (*agentstep.ToolResult, error) {
	if !strings.EqualFold(mode, "run") {
		return nil, fmt.Errorf("unsupported command mode %q", mode)
	}

	var input commandParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse command params: %w", err)
	}
	if strings.TrimSpace(input.Command) == "" {
		return nil, fmt.Errorf("command is required")
	}

	workdir := r.workspaceRoot
	if strings.TrimSpace(input.Workdir) != "" {
		resolved, err := r.resolveWorkspacePath(input.Workdir)
		if err != nil {
			return nil, err
		}
		workdir = resolved
	}

	timeout := time.Duration(0)
	if strings.TrimSpace(input.Timeout) != "" {
		parsed, err := time.ParseDuration(input.Timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timeout: %w", err)
		}
		timeout = parsed
	}

	cmdCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := buildShellCommand(cmdCtx, input.Command)
	cmd.Dir = workdir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("command failed: %w", err)
	}

	r.workspaceDirty = true

	return &agentstep.ToolResult{
		Tool: "command",
		Mode: "run",
		Data: map[string]string{
			"stdout": strings.TrimSpace(stdout.String()),
			"stderr": strings.TrimSpace(stderr.String()),
		},
	}, nil
}

func buildShellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		cmd := exec.CommandContext(ctx, "cmd", "/Q", "/D")
		cmd.Stdin = strings.NewReader(command + "\nexit\n")
		return cmd
	}

	cmd := exec.CommandContext(ctx, "sh")
	cmd.Stdin = strings.NewReader(command + "\n")
	return cmd
}

type patchParams struct {
	Diff string `json:"diff"`
}

func (r *ToolRunner) runPatchTool(ctx context.Context, _ *executor.Context, mode string, params json.RawMessage) (*agentstep.ToolResult, error) {
	if !strings.EqualFold(mode, "apply") {
		return nil, fmt.Errorf("unsupported patch mode %q", mode)
	}

	var input patchParams
	if err := json.Unmarshal(params, &input); err != nil {
		return nil, fmt.Errorf("failed to parse patch params: %w", err)
	}
	if strings.TrimSpace(input.Diff) == "" {
		return nil, fmt.Errorf("diff is required")
	}

	cmd := exec.CommandContext(ctx, "git", "apply", "--whitespace=nowarn")
	cmd.Dir = r.workspaceRoot
	cmd.Stdin = strings.NewReader(input.Diff)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to apply patch: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	r.workspaceDirty = true

	return &agentstep.ToolResult{Tool: "patch", Mode: "apply", Data: map[string]string{"status": "applied"}}, nil
}
