# Bubble Tea UI Patterns and Components

This document outlines UI patterns, components, and theming used in meowg1k's terminal interface.

## UI Stack

meowg1k uses the following libraries for terminal UI:

- **Bubble Tea** - TUI framework for interactive components
- **Lip Gloss** - Terminal styling and layout
- **Glamour** - Markdown rendering
- **Chroma** - Syntax highlighting

## Theme System

### Theme Structure

Themes are defined in `internal/ui/theme.go`:

```go
type Theme struct {
    // Base colors
    Text       lipgloss.Color
    Muted      lipgloss.Color
    Accent     lipgloss.Color
    Success    lipgloss.Color
    Error      lipgloss.Color
    Border     lipgloss.Color
    
    // Flux Terminal semantic colors
    System      lipgloss.Color  // System/infrastructure messages
    Agent       lipgloss.Color  // AI/LLM operations
    Action      lipgloss.Color  // Tool calls, external actions
    Thought     lipgloss.Color  // Agent reasoning (dimmed)
    Spinner     lipgloss.Color  // Activity spinner
    InputPrompt lipgloss.Color  // User input prompts
    
    // Pre-built styles
    StatusSuccess   lipgloss.Style
    StatusError     lipgloss.Style
    PanelStyle      lipgloss.Style
    TableHeader     lipgloss.Style
}
```

### Semantic Colors

**Flux Terminal** semantic color system for AI agent output:

- **System** (SlateGray) - System/infrastructure messages
- **Agent** (Magenta) - AI/LLM operations and responses
- **Action** (Cyan) - Tool calls, external actions
- **Thought** (Dimmed) - Agent reasoning and planning
- **Spinner** (Teal) - Activity indicators
- **InputPrompt** (White Bold) - User input prompts

### Using Theme Colors

```go
func renderMessage(theme Theme, message string) string {
    return theme.Agent.Render(message)
}

func renderError(theme Theme, err error) string {
    return theme.Error.Render(fmt.Sprintf("Error: %v", err))
}
```

## Render Options

Control rendering behavior based on terminal capabilities:

```go
type RenderOptions struct {
    Plain           bool  // No borders, no colors, no padding
    NoColor         bool  // No ANSI colors
    NoEmoji         bool  // Use text instead of emoji
    Terminal        bool  // Output is going to a terminal
    SupportsUnicode bool  // Terminal supports Unicode
}

// Auto-detect terminal capabilities
opts := ui.NewRenderOptions()
```

## Core Components

### 1. Markdown Rendering

Render markdown with syntax highlighting:

```go
// internal/ui/markdown.go
func RenderMarkdown(content string, opts RenderOptions) (string, error) {
    renderer, err := glamour.NewTermRenderer(
        glamour.WithAutoStyle(),
        glamour.WithWordWrap(80),
    )
    if err != nil {
        return "", err
    }
    return renderer.Render(content)
}
```

**Usage**:

```go
rendered, err := ui.RenderMarkdown("# Heading\n\nParagraph", opts)
```

### 2. Code Highlighting

Syntax-highlighted code blocks:

```go
// internal/ui/code.go
func RenderCode(code, language string, theme Theme) string {
    lexer := lexers.Get(language)
    if lexer == nil {
        lexer = lexers.Fallback
    }
    
    formatter := formatters.Get("terminal256")
    style := styles.Get("monokai")
    
    var buf bytes.Buffer
    iterator, _ := lexer.Tokenise(nil, code)
    formatter.Format(&buf, style, iterator)
    
    return buf.String()
}
```

**Usage**:

```go
code := "func main() { println(\"hello\") }"
rendered := ui.RenderCode(code, "go", theme)
```

### 3. Diff Visualization

Display Git-style diffs:

```go
// internal/ui/diff.go
func RenderDiff(diff string, theme Theme, opts RenderOptions) string {
    lines := strings.Split(diff, "\n")
    var result strings.Builder
    
    for _, line := range lines {
        switch {
        case strings.HasPrefix(line, "+"):
            result.WriteString(theme.DiffAdd.Render(line))
        case strings.HasPrefix(line, "-"):
            result.WriteString(theme.DiffDel.Render(line))
        case strings.HasPrefix(line, "@@"):
            result.WriteString(theme.DiffHunk.Render(line))
        default:
            result.WriteString(line)
        }
        result.WriteString("\n")
    }
    
    return result.String()
}
```

### 4. Progress Bar

Visual progress indicator:

```go
// internal/ui/progress_bar.go
type ProgressBar struct {
    Width     int
    Percent   float64
    ShowLabel bool
    Theme     Theme
}

func (p *ProgressBar) Render() string {
    filled := int(float64(p.Width) * p.Percent)
    empty := p.Width - filled
    
    bar := strings.Repeat("█", filled) + 
           strings.Repeat("░", empty)
    
    if p.ShowLabel {
        label := fmt.Sprintf(" %.0f%%", p.Percent*100)
        return p.Theme.Accent.Render(bar) + label
    }
    
    return p.Theme.Accent.Render(bar)
}
```

**Usage**:

```go
bar := ui.ProgressBar{
    Width:     40,
    Percent:   0.75,
    ShowLabel: true,
    Theme:     theme,
}
fmt.Println(bar.Render())  // ████████████████████████████████░░░░░░░░ 75%
```

### 5. Activity Spinner

Animated spinner for long operations:

```go
// internal/ui/activity.go
type Activity struct {
    Message string
    Theme   Theme
    frame   int
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (a *Activity) Tick() {
    a.frame = (a.frame + 1) % len(spinnerFrames)
}

func (a *Activity) Render() string {
    spinner := a.Theme.Spinner.Render(spinnerFrames[a.frame])
    return fmt.Sprintf("%s %s", spinner, a.Message)
}
```

### 6. Tables

Tabular data display:

```go
// internal/ui/table.go
func RenderTable(headers []string, rows [][]string, theme Theme) string {
    // Calculate column widths
    widths := make([]int, len(headers))
    for i, h := range headers {
        widths[i] = len(h)
    }
    for _, row := range rows {
        for i, cell := range row {
            if len(cell) > widths[i] {
                widths[i] = len(cell)
            }
        }
    }
    
    // Render header
    var result strings.Builder
    for i, h := range headers {
        padded := h + strings.Repeat(" ", widths[i]-len(h))
        result.WriteString(theme.TableHeader.Render(padded))
        result.WriteString("  ")
    }
    result.WriteString("\n")
    
    // Render rows
    for _, row := range rows {
        for i, cell := range row {
            padded := cell + strings.Repeat(" ", widths[i]-len(cell))
            result.WriteString(theme.TableRow.Render(padded))
            result.WriteString("  ")
        }
        result.WriteString("\n")
    }
    
    return result.String()
}
```

### 7. Selection Menu (Bubble Tea)

Interactive selection using Bubble Tea:

```go
// internal/ui/select.go
type SelectModel struct {
    choices  []string
    cursor   int
    selected map[int]struct{}
    Theme    Theme
}

func (m SelectModel) Init() tea.Cmd {
    return nil
}

func (m SelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "up", "k":
            if m.cursor > 0 {
                m.cursor--
            }
        case "down", "j":
            if m.cursor < len(m.choices)-1 {
                m.cursor++
            }
        case "enter", " ":
            m.selected[m.cursor] = struct{}{}
        }
    }
    return m, nil
}

func (m SelectModel) View() string {
    var s strings.Builder
    
    for i, choice := range m.choices {
        cursor := " "
        if m.cursor == i {
            cursor = m.Theme.SelectCursor.Render(">")
        }
        
        checked := " "
        if _, ok := m.selected[i]; ok {
            checked = m.Theme.Success.Render("✓")
        }
        
        s.WriteString(fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice))
    }
    
    return s.String()
}
```

### 8. Panels

Bordered containers for content:

```go
// internal/ui/panel.go
func RenderPanel(title, content string, theme Theme, opts RenderOptions) string {
    if opts.Plain || opts.NoBorders {
        return content
    }
    
    border := lipgloss.RoundedBorder()
    style := lipgloss.NewStyle().
        Border(border).
        BorderForeground(theme.Border).
        Padding(1, 2)
    
    if title != "" {
        style = style.BorderTop(true).
            BorderTitle(theme.PanelTitleStyle.Render(title))
    }
    
    return style.Render(content)
}
```

### 9. Banners

Prominent text headers:

```go
// internal/ui/banner.go
func RenderBanner(text string, theme Theme) string {
    style := lipgloss.NewStyle().
        Bold(true).
        Foreground(theme.Accent).
        Padding(1, 0)
    
    return style.Render(text)
}
```

### 10. Links

Hyperlinks (terminal support required):

```go
// internal/ui/link.go
func RenderLink(text, url string, theme Theme) string {
    // OSC 8 hyperlink format: \x1b]8;;URL\x1bTEXT\x1b]8;;\x1b\
    if SupportsHyperlinks() {
        return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\",
            url,
            theme.Accent.Render(text))
    }
    return theme.Accent.Render(fmt.Sprintf("%s (%s)", text, url))
}
```

## Layout Patterns

### Horizontal Layout

```go
func horizontalLayout(left, right string) string {
    return lipgloss.JoinHorizontal(
        lipgloss.Top,
        left,
        right,
    )
}
```

### Vertical Layout

```go
func verticalLayout(top, bottom string) string {
    return lipgloss.JoinVertical(
        lipgloss.Left,
        top,
        bottom,
    )
}
```

### Center Alignment

```go
func centerAlign(content string, width int) string {
    style := lipgloss.NewStyle().
        Width(width).
        Align(lipgloss.Center)
    return style.Render(content)
}
```

## Interactive Patterns

### Prompt for Input

```go
func PromptInput(message string, theme Theme) (string, error) {
    fmt.Print(theme.InputPrompt.Render(message + ": "))
    
    reader := bufio.NewReader(os.Stdin)
    input, err := reader.ReadString('\n')
    if err != nil {
        return "", err
    }
    
    return strings.TrimSpace(input), nil
}
```

### Confirmation Dialog

```go
func Confirm(message string, theme Theme) bool {
    fmt.Print(theme.InputPrompt.Render(message + " (y/n): "))
    
    var response string
    fmt.Scanln(&response)
    
    return strings.ToLower(response) == "y" || 
           strings.ToLower(response) == "yes"
}
```

## Best Practices

### 1. Respect RenderOptions

Always check render options before applying styling:

```go
func render(content string, theme Theme, opts RenderOptions) string {
    if opts.Plain {
        return content  // No styling
    }
    if opts.NoColor {
        // Apply structure but no colors
    }
    return theme.Text.Render(content)
}
```

### 2. Handle Non-Terminal Output

Check if output is going to a terminal:

```go
if !opts.Terminal {
    // Simple output without escape codes
    return plainText
}
```

### 3. Graceful Degradation

Provide text fallbacks for Unicode:

```go
func checkMark(opts RenderOptions) string {
    if opts.NoEmoji || !opts.SupportsUnicode {
        return "[OK]"
    }
    return "✓"
}
```

### 4. Consistent Spacing

Use consistent padding and margins:

```go
style := lipgloss.NewStyle().
    Padding(1, 2).      // Vertical, horizontal
    Margin(0, 0, 1, 0)  // Top, right, bottom, left
```

### 5. Theme Consistency

Use theme colors consistently:

```go
// ✅ GOOD: Use semantic colors
errorMsg := theme.Error.Render("Failed")
successMsg := theme.Success.Render("Done")

// ❌ BAD: Hardcoded colors
errorMsg := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Render("Failed")
```

## Testing UI Components

### Unit Tests

Test rendering logic without actual terminal:

```go
func TestProgressBar_Render(t *testing.T) {
    bar := ProgressBar{
        Width:   10,
        Percent: 0.5,
        Theme:   DefaultTheme(),
    }
    
    result := bar.Render()
    
    assert.Contains(t, result, "█████")
    assert.Contains(t, result, "░░░░░")
}
```

### Visual Tests

Create visual test files for manual inspection:

```go
// debug_ui.go
func main() {
    theme := DefaultTheme()
    
    fmt.Println(RenderBanner("meowg1k", theme))
    fmt.Println(RenderPanel("Info", "Content", theme, RenderOptions{}))
    
    bar := ProgressBar{Width: 40, Percent: 0.75, Theme: theme}
    fmt.Println(bar.Render())
}
```

## Component Checklist

When creating new UI components:

- [ ] Respect `RenderOptions` (Plain, NoColor, NoBorders)
- [ ] Use theme colors (not hardcoded)
- [ ] Handle non-terminal output
- [ ] Provide Unicode/emoji fallbacks
- [ ] Add godoc comments
- [ ] Write unit tests
- [ ] Test with different terminal emulators

## Summary

- **Theme system** for consistent colors and styles
- **Semantic colors** for AI agent output (Flux Terminal)
- **RenderOptions** for terminal capability detection
- **Rich components**: markdown, code, diff, progress, tables, selection
- **Bubble Tea** for interactive components
- **Graceful degradation** for non-terminal output
- **Consistent styling** via Lip Gloss

Follow these patterns to maintain a polished, consistent terminal UI across meowg1k.
