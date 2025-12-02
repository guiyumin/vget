# Zsh Completion Limit

## Issue

When using zsh shell completion for remote paths (e.g., `pikpak:/电影/`), directories with many files (50+) caused the cursor to move to a new prompt line after displaying completions. This made it impossible to continue typing to filter or select a completion.

Directories with fewer files (< 30) worked correctly - the cursor stayed on the same line, allowing continued interaction.

## Root Cause

Zsh has an internal behavior threshold related to the number of completion items displayed. When the completion list exceeds approximately 40-50 items, zsh redraws the prompt after showing completions, which moves the cursor to a new line.

This is a zsh behavior, not a Cobra or vget bug. Testing showed:
- 20 items: cursor stays (works)
- 30 items: cursor stays (works)
- 50 items: cursor moves to new line (broken)

The exact threshold may vary based on terminal size or zsh configuration, but appears to be in the 40-50 range.

## Solution

Limit the number of completions returned to 15 items. This provides a safe margin below the threshold.

Users with directories containing more than 15 files can simply type a few additional characters to filter the results before pressing Tab.

## Code Change

```go
// internal/cli/completion.go

// Limit completions to avoid zsh prompt redraw issue with large lists
// zsh redraws prompt when showing too many completions (threshold varies)
// Limit to 15 for safe margin; users can type more chars to filter
const maxCompletions = 15
if len(completions) > maxCompletions {
    completions = completions[:maxCompletions]
}
```

## Testing

1. `pikpak:/` (root) - works, shows directories
2. `pikpak:/电影/` (68 files) - now shows first 15, cursor stays on line
3. `pikpak:/鱿鱼游戏/` (7 files) - works, shows all files
4. Type partial name + Tab filters and completes correctly
