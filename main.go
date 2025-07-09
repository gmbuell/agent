package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	anthropicAPIURL = "https://api.anthropic.com/v1/messages"
	model           = "claude-3-opus-20240229"
	maxTokens       = 4000
	temperature     = 0.1
)

type ToolSchema struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"input_schema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Message struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type ContentBlock struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

type ToolResult struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
}

type APIRequest struct {
	Model       string       `json:"model"`
	MaxTokens   int          `json:"max_tokens"`
	Messages    []Message    `json:"messages"`
	System      string       `json:"system"`
	Tools       []ToolSchema `json:"tools"`
	Temperature float64      `json:"temperature"`
}

type APIResponse struct {
	Content []ContentBlock `json:"content"`
}

type ShellResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// Global state to track sed dry-run operations
var sedDryRunCache = make(map[string]bool)

func executeShellCommand(command string, timeout time.Duration) ShellResult {
	fmt.Printf("\n🔧 Executing shell command: %s\n", command)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ShellResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Printf("\n⏱️ Command timed out after %v\n", timeout)
		}
		result.ExitCode = 1
		if result.Stderr == "" {
			result.Stderr = err.Error()
		}
	}

	return result
}

func executeGoDoc(packageOrSymbol string, timeout time.Duration) ShellResult {
	fmt.Printf("\n📚 Executing go doc: %s\n", packageOrSymbol)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "doc", packageOrSymbol)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ShellResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Printf("\n⏱️ Go doc command timed out after %v\n", timeout)
		}
		result.ExitCode = 1
		if result.Stderr == "" {
			result.Stderr = err.Error()
		}
	}

	return result
}

func executeRipgrep(pattern, path string, ignoreCase, lineNumbers, filesWithMatches bool, timeout time.Duration) ShellResult {
	fmt.Printf("\n🔍 Executing ripgrep: %s\n", pattern)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"rg"}

	if ignoreCase {
		args = append(args, "-i")
	}
	if lineNumbers {
		args = append(args, "-n")
	}
	if filesWithMatches {
		args = append(args, "-l")
	}

	args = append(args, pattern)

	if path != "" {
		args = append(args, path)
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ShellResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Printf("\n⏱️ Ripgrep command timed out after %v\n", timeout)
		}
		result.ExitCode = 1
		if result.Stderr == "" {
			result.Stderr = err.Error()
		}
	}

	return result
}

func generateSedOperationKey(filePath, searchPattern, replacePattern string) string {
	data := fmt.Sprintf("%s|%s|%s", filePath, searchPattern, replacePattern)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func executeGofmt(target string, listFiles, diff, write bool, timeout time.Duration) ShellResult {
	fmt.Printf("\n🔧 Formatting Go code: %s\n", target)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd

	// For listing files or showing diff, we need to use gofmt directly
	if listFiles || diff {
		args := []string{"gofmt"}

		if listFiles {
			args = append(args, "-l")
		}
		if diff {
			args = append(args, "-d")
		}

		// Add target (file, directory, or pattern)
		if target != "" {
			args = append(args, target)
		} else {
			// Default to current directory
			args = append(args, "./...")
		}

		cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	} else {
		// For formatting in place or just checking
		// Handle wildcard patterns or directories by using gofmt instead of go fmt
		if strings.Contains(target, "*") || write || strings.HasSuffix(target, "/") {
			args := []string{"gofmt"}
			if write {
				args = append(args, "-w")
			}

			if target != "" {
				args = append(args, target)
			} else {
				// Default to current directory
				args = append(args, "./...")
			}

			cmd = exec.CommandContext(ctx, args[0], args[1:]...)
		} else {
			// Use go fmt for package-based formatting
			args := []string{"go", "fmt"}

			// Add target (file, directory, or pattern)
			if target != "" {
				args = append(args, target)
			} else {
				// Default to current directory
				args = append(args, "./...")
			}

			cmd = exec.CommandContext(ctx, args[0], args[1:]...)
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ShellResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Printf("\n⏱️ Go fmt command timed out after %v\n", timeout)
		}
		result.ExitCode = 1
		if result.Stderr == "" {
			result.Stderr = err.Error()
		}
	}

	return result
}

func executeAsk(question string) ShellResult {
	fmt.Printf("\n❓ Agent is asking for clarification:\n%s\n", question)
	fmt.Print("👤 Your response: ")

	var response string
	fmt.Scanln(&response)

	return ShellResult{
		Stdout:   response,
		Stderr:   "",
		ExitCode: 0,
	}
}

func executeComby(matchTemplate, rewriteTemplate, target string, matchOnly, inPlace, diff bool, language, rule string, timeout time.Duration) ShellResult {
	fmt.Printf("\n🔀 Executing comby: %s\n", matchTemplate)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"comby"}

	// Add match template
	args = append(args, matchTemplate)

	// Add rewrite template (empty for match-only)
	if matchOnly {
		args = append(args, "")
		args = append(args, "-match-only")
	} else {
		args = append(args, rewriteTemplate)
	}

	// Add target (files, directories, or extensions)
	if target != "" {
		args = append(args, target)
	}

	// Add language/matcher if specified
	if language != "" {
		args = append(args, "-matcher", language)
	}

	// Add rule if specified
	if rule != "" {
		args = append(args, "-rule", rule)
	}

	// Add options
	if inPlace {
		args = append(args, "-in-place")
	}
	if diff {
		args = append(args, "-diff")
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ShellResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Printf("\n⏱️ Comby command timed out after %v\n", timeout)
		}
		result.ExitCode = 1
		if result.Stderr == "" {
			result.Stderr = err.Error()
		}
	}

	return result
}

func executeTodo(action, filePath, content string, timeout time.Duration) ShellResult {
	switch action {
	case "read":
		return readTodoFile(filePath)
	case "write":
		return writeTodoFile(filePath, content)
	case "add":
		return addTodoItem(filePath, content)
	case "complete":
		return completeTodoItem(filePath, content)
	case "update":
		return updateTodoItem(filePath, content)
	default:
		return ShellResult{
			Stdout:   "",
			Stderr:   fmt.Sprintf("Unknown todo action: %s", action),
			ExitCode: 1,
		}
	}
}

func readTodoFile(filePath string) ShellResult {
	fmt.Printf("\n📝 Reading todo file: %s\n", filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ShellResult{
				Stdout:   "",
				Stderr:   fmt.Sprintf("Todo file does not exist: %s", filePath),
				ExitCode: 1,
			}
		}
		return ShellResult{
			Stdout:   "",
			Stderr:   fmt.Sprintf("Error reading file: %v", err),
			ExitCode: 1,
		}
	}

	return ShellResult{
		Stdout:   string(content),
		Stderr:   "",
		ExitCode: 0,
	}
}

func writeTodoFile(filePath, content string) ShellResult {
	fmt.Printf("\n✏️ Writing todo file: %s\n", filePath)

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return ShellResult{
			Stdout:   "",
			Stderr:   fmt.Sprintf("Error writing file: %v", err),
			ExitCode: 1,
		}
	}

	return ShellResult{
		Stdout:   "Todo file written successfully",
		Stderr:   "",
		ExitCode: 0,
	}
}

func addTodoItem(filePath, item string) ShellResult {
	fmt.Printf("\n➕ Adding todo item to: %s\n", filePath)

	// Read existing content or create new file
	var existingContent string
	if content, err := os.ReadFile(filePath); err == nil {
		existingContent = string(content)
	} else if !os.IsNotExist(err) {
		return ShellResult{
			Stdout:   "",
			Stderr:   fmt.Sprintf("Error reading file: %v", err),
			ExitCode: 1,
		}
	}

	// Add header if file is empty
	if strings.TrimSpace(existingContent) == "" {
		existingContent = "# Todo List\n\n"
	}

	// Add new item
	newContent := existingContent
	if !strings.HasSuffix(existingContent, "\n") {
		newContent += "\n"
	}
	newContent += fmt.Sprintf("- [ ] %s\n", item)

	err := os.WriteFile(filePath, []byte(newContent), 0644)
	if err != nil {
		return ShellResult{
			Stdout:   "",
			Stderr:   fmt.Sprintf("Error writing file: %v", err),
			ExitCode: 1,
		}
	}

	return ShellResult{
		Stdout:   "Todo item added successfully",
		Stderr:   "",
		ExitCode: 0,
	}
}

func completeTodoItem(filePath, item string) ShellResult {
	fmt.Printf("\n✅ Completing todo item in: %s\n", filePath)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return ShellResult{
			Stdout:   "",
			Stderr:   fmt.Sprintf("Error reading file: %v", err),
			ExitCode: 1,
		}
	}

	// Find and mark item as complete
	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.Contains(line, item) && strings.Contains(line, "- [ ]") {
			lines[i] = strings.Replace(line, "- [ ]", "- [x]", 1)
			found = true
			break
		}
	}

	if !found {
		return ShellResult{
			Stdout:   "",
			Stderr:   fmt.Sprintf("Todo item not found: %s", item),
			ExitCode: 1,
		}
	}

	newContent := strings.Join(lines, "\n")
	err = os.WriteFile(filePath, []byte(newContent), 0644)
	if err != nil {
		return ShellResult{
			Stdout:   "",
			Stderr:   fmt.Sprintf("Error writing file: %v", err),
			ExitCode: 1,
		}
	}

	return ShellResult{
		Stdout:   "Todo item completed successfully",
		Stderr:   "",
		ExitCode: 0,
	}
}

func updateTodoItem(filePath, update string) ShellResult {
	fmt.Printf("\n🔄 Updating todo item in: %s\n", filePath)

	// Parse update format: "old_item -> new_item"
	parts := strings.Split(update, " -> ")
	if len(parts) != 2 {
		return ShellResult{
			Stdout:   "",
			Stderr:   "Update format should be: 'old_item -> new_item'",
			ExitCode: 1,
		}
	}

	oldItem := strings.TrimSpace(parts[0])
	newItem := strings.TrimSpace(parts[1])

	content, err := os.ReadFile(filePath)
	if err != nil {
		return ShellResult{
			Stdout:   "",
			Stderr:   fmt.Sprintf("Error reading file: %v", err),
			ExitCode: 1,
		}
	}

	// Find and update item
	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.Contains(line, oldItem) && (strings.Contains(line, "- [ ]") || strings.Contains(line, "- [x]")) {
			// Preserve the checkbox state
			if strings.Contains(line, "- [ ]") {
				lines[i] = fmt.Sprintf("- [ ] %s", newItem)
			} else {
				lines[i] = fmt.Sprintf("- [x] %s", newItem)
			}
			found = true
			break
		}
	}

	if !found {
		return ShellResult{
			Stdout:   "",
			Stderr:   fmt.Sprintf("Todo item not found: %s", oldItem),
			ExitCode: 1,
		}
	}

	newContent := strings.Join(lines, "\n")
	err = os.WriteFile(filePath, []byte(newContent), 0644)
	if err != nil {
		return ShellResult{
			Stdout:   "",
			Stderr:   fmt.Sprintf("Error writing file: %v", err),
			ExitCode: 1,
		}
	}

	return ShellResult{
		Stdout:   "Todo item updated successfully",
		Stderr:   "",
		ExitCode: 0,
	}
}

func executeSed(filePath, searchPattern, replacePattern string, dryRun bool, timeout time.Duration) ShellResult {
	operationKey := generateSedOperationKey(filePath, searchPattern, replacePattern)

	if dryRun {
		fmt.Printf("\n🔍 Sed dry-run on %s: s/%s/%s/g\n", filePath, searchPattern, replacePattern)
	} else {
		// Check if dry-run was performed for this exact operation
		if !sedDryRunCache[operationKey] {
			return ShellResult{
				Stdout:   "",
				Stderr:   "ERROR: Must perform dry-run before applying sed changes. Please run the same sed command with dryRun=true first.",
				ExitCode: 1,
			}
		}
		fmt.Printf("\n✏️ Sed applying changes to %s: s/%s/%s/g\n", filePath, searchPattern, replacePattern)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	if dryRun {
		// First check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return ShellResult{
				Stdout:   "",
				Stderr:   fmt.Sprintf("sed: %s: No such file or directory", filePath),
				ExitCode: 1,
			}
		}
		// Create a temporary file for the modified content and show diff
		tempFile := filePath + ".tmp"
		sedCmd := fmt.Sprintf("sed 's/%s/%s/g' '%s' > '%s' && diff -u '%s' '%s'; rm -f '%s'",
			searchPattern, replacePattern, filePath, tempFile, filePath, tempFile, tempFile)
		cmd = exec.CommandContext(ctx, "sh", "-c", sedCmd)
	} else {
		// Apply changes in-place
		cmd = exec.CommandContext(ctx, "sed", "-i", fmt.Sprintf("s/%s/%s/g", searchPattern, replacePattern), filePath)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ShellResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Printf("\n⏱️ Sed command timed out after %v\n", timeout)
		}
		result.ExitCode = 1
		if result.Stderr == "" {
			result.Stderr = err.Error()
		}
	}

	// If dry-run was successful, mark it as completed
	if dryRun && result.ExitCode == 0 {
		sedDryRunCache[operationKey] = true
	} else if !dryRun && result.ExitCode == 0 {
		// Clear the dry-run cache entry after successful application
		delete(sedDryRunCache, operationKey)
	}

	return result
}

func callAnthropicAPI(request APIRequest) (*APIResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", anthropicAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is required")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

func runAgentLoop(initialPrompt string) error {
	shellCommandSchema := ToolSchema{
		Name:        "shellCommand",
		Description: "Execute a shell command and return the result",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"command": {
					Type:        "string",
					Description: "The shell command to execute",
				},
			},
			Required: []string{"command"},
		},
	}

	goDocSchema := ToolSchema{
		Name:        "goDoc",
		Description: "Execute go doc command to get documentation for Go packages, types, or functions",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"packageOrSymbol": {
					Type:        "string",
					Description: "The package, type, or function to get documentation for (e.g., 'fmt', 'fmt.Println', 'net/http')",
				},
			},
			Required: []string{"packageOrSymbol"},
		},
	}

	ripgrepSchema := ToolSchema{
		Name:        "ripgrep",
		Description: "Search for patterns in files using ripgrep (rg)",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"pattern": {
					Type:        "string",
					Description: "The regular expression pattern to search for",
				},
				"path": {
					Type:        "string",
					Description: "Optional path to search in (defaults to current directory)",
				},
				"ignoreCase": {
					Type:        "boolean",
					Description: "Perform case-insensitive search (default: false)",
				},
				"lineNumbers": {
					Type:        "boolean",
					Description: "Show line numbers in output (default: false)",
				},
				"filesWithMatches": {
					Type:        "boolean",
					Description: "Only show file paths with matches (default: false)",
				},
			},
			Required: []string{"pattern"},
		},
	}

	sedSchema := ToolSchema{
		Name:        "sed",
		Description: "Search and replace text in files using sed. ENFORCED: You must ALWAYS do a dry-run (dryRun=true) first to show diff before applying changes (dryRun=false). The system will reject apply operations without a prior dry-run.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"filePath": {
					Type:        "string",
					Description: "Path to the file to modify",
				},
				"searchPattern": {
					Type:        "string",
					Description: "Pattern to search for (regex supported)",
				},
				"replacePattern": {
					Type:        "string",
					Description: "Text to replace with",
				},
				"dryRun": {
					Type:        "boolean",
					Description: "REQUIRED: Must be true first for preview, then false to apply. System enforces dry-run before apply.",
				},
			},
			Required: []string{"filePath", "searchPattern", "replacePattern", "dryRun"},
		},
	}

	todoSchema := ToolSchema{
		Name:        "todo",
		Description: "Manage todo.md files for planning and tracking multi-step changes",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"action": {
					Type:        "string",
					Description: "Action to perform: 'read', 'write', 'add', 'complete', or 'update'",
				},
				"filePath": {
					Type:        "string",
					Description: "Path to the todo.md file (default: todo.md)",
				},
				"content": {
					Type:        "string",
					Description: "Content for the action (item text, full content, or 'old -> new' for update)",
				},
			},
			Required: []string{"action"},
		},
	}

	combySchema := ToolSchema{
		Name:        "comby",
		Description: "Advanced structural search and replace tool for code. Comby uses template-based matching with holes (:[name]) to match code structurally, understanding balanced delimiters, comments, and strings. Examples: 'fmt.Println(:[args])' matches function calls, 'if (:[condition]) { :[body] }' matches if statements. Supports regex in holes with :[name~regex] syntax. Can match-only or rewrite code in-place with diff preview.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"matchTemplate": {
					Type:        "string",
					Description: "Template to match code structure using holes like :[name]. Example: 'fmt.Println(:[args])' or 'if (:[condition]) { :[body] }'",
				},
				"rewriteTemplate": {
					Type:        "string",
					Description: "Template to rewrite matched code. Use same hole names as match template. Example: 'log.Printf(\"msg: %s\", :[args])'",
				},
				"target": {
					Type:        "string",
					Description: "Target files/directories/extensions to search. Examples: '.go', 'src/', 'main.go', '.js,.ts'",
				},
				"matchOnly": {
					Type:        "boolean",
					Description: "Only find matches without rewriting (default: false)",
				},
				"inPlace": {
					Type:        "boolean",
					Description: "Rewrite files in place (default: false)",
				},
				"diff": {
					Type:        "boolean",
					Description: "Show diff of changes (default: false)",
				},
				"language": {
					Type:        "string",
					Description: "Force language/matcher: .go, .js, .py, .java, .c, .generic, etc.",
				},
				"rule": {
					Type:        "string",
					Description: "Apply rules to matches (advanced). Example: 'where match.var == \"foo\"'",
				},
			},
			Required: []string{"matchTemplate"},
		},
	}

	gofmtSchema := ToolSchema{
		Name:        "gofmt",
		Description: "Format Go source code using 'go fmt'. IMPORTANT: Agent should run this regularly after creating or modifying Go files to maintain proper formatting. Use write=true to format files in-place.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"target": {
					Type:        "string",
					Description: "Target to format: file path, directory, or pattern (default: './...' for all Go files)",
				},
				"listFiles": {
					Type:        "boolean",
					Description: "List files that would be formatted without formatting them (default: false)",
				},
				"diff": {
					Type:        "boolean",
					Description: "Show diff of formatting changes without applying them (default: false)",
				},
				"write": {
					Type:        "boolean",
					Description: "Write formatted code back to files in-place (default: false)",
				},
			},
			Required: []string{},
		},
	}

	askSchema := ToolSchema{
		Name:        "ask",
		Description: "Ask the user for clarification or additional information when the agent needs input to proceed. Use this when requirements are unclear, multiple options exist, or user preferences are needed.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"question": {
					Type:        "string",
					Description: "The question or clarification request to present to the user. Be specific and clear about what information you need.",
				},
			},
			Required: []string{"question"},
		},
	}

	finishedSchema := ToolSchema{
		Name:        "finished",
		Description: "Call this tool when the task is complete to end the conversation",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
			Required:   []string{},
		},
	}

	tools := []ToolSchema{shellCommandSchema, goDocSchema, ripgrepSchema, sedSchema, todoSchema, combySchema, gofmtSchema, askSchema, finishedSchema}

	systemPrompt := "You are an AI agent that can run shell commands, access Go documentation, search files, edit text files, perform structural code search/replace, format Go code, ask for user clarification, and manage todo lists to accomplish tasks.\n" +
		"Use the provided tools to complete the user's task:\n" +
		"- shellCommand: Execute any shell command\n" +
		"- goDoc: Get documentation for Go packages, types, or functions\n" +
		"- ripgrep: Search for patterns in files using ripgrep (fast file search)\n" +
		"- sed: Search and replace text in files. MANDATORY: You MUST do dry-run (dryRun=true) first, then apply (dryRun=false). System enforces this workflow.\n" +
		"- todo: Manage todo.md files for planning multi-step changes (actions: read, write, add, complete, update)\n" +
		"- comby: Advanced structural search and replace for code using templates with holes (:[name]). Understands language syntax, balanced delimiters, comments, strings. Use for complex code transformations.\n" +
		"- gofmt: Format Go source code. IMPORTANT: You should run this regularly after creating or modifying Go files to maintain proper formatting. Use write=true to format in-place.\n" +
		"- ask: Ask the user for clarification when requirements are unclear, multiple options exist, or user preferences are needed. Use this to get specific guidance before proceeding.\n" +
		"When the task is complete, call the finished tool to indicate completion."

	messages := []Message{
		{
			Role:    "user",
			Content: initialPrompt,
		},
	}

	for {
		fmt.Println("\n🤔 Thinking...")

		request := APIRequest{
			Model:       model,
			MaxTokens:   maxTokens,
			Messages:    messages,
			System:      systemPrompt,
			Tools:       tools,
			Temperature: temperature,
		}

		response, err := callAnthropicAPI(request)
		if err != nil {
			return fmt.Errorf("API call failed: %w", err)
		}

		messages = append(messages, Message{
			Role:    "assistant",
			Content: response.Content,
		})

		hasToolUse := false

		for _, block := range response.Content {
			if block.Type == "text" {
				fmt.Printf("\n🤖 %s\n", block.Text)
				continue
			}

			if block.Type == "tool_use" {
				hasToolUse = true
				switch block.Name {
				case "shellCommand":
					if cmd, ok := block.Input["command"].(string); ok {
						result := executeShellCommand(cmd, 10*time.Second)
						resultJSON, _ := json.Marshal(result)

						messages = append(messages, Message{
							Role: "user",
							Content: []ToolResult{
								{
									Type:      "tool_result",
									ToolUseID: block.ID,
									Content:   string(resultJSON),
								},
							},
						})
					}
				case "goDoc":
					if packageOrSymbol, ok := block.Input["packageOrSymbol"].(string); ok {
						result := executeGoDoc(packageOrSymbol, 10*time.Second)
						resultJSON, _ := json.Marshal(result)

						messages = append(messages, Message{
							Role: "user",
							Content: []ToolResult{
								{
									Type:      "tool_result",
									ToolUseID: block.ID,
									Content:   string(resultJSON),
								},
							},
						})
					}
				case "ripgrep":
					if pattern, ok := block.Input["pattern"].(string); ok {
						path, _ := block.Input["path"].(string)
						ignoreCase, _ := block.Input["ignoreCase"].(bool)
						lineNumbers, _ := block.Input["lineNumbers"].(bool)
						filesWithMatches, _ := block.Input["filesWithMatches"].(bool)

						result := executeRipgrep(pattern, path, ignoreCase, lineNumbers, filesWithMatches, 10*time.Second)
						resultJSON, _ := json.Marshal(result)

						messages = append(messages, Message{
							Role: "user",
							Content: []ToolResult{
								{
									Type:      "tool_result",
									ToolUseID: block.ID,
									Content:   string(resultJSON),
								},
							},
						})
					}
				case "sed":
					if filePath, ok := block.Input["filePath"].(string); ok {
						if searchPattern, ok := block.Input["searchPattern"].(string); ok {
							if replacePattern, ok := block.Input["replacePattern"].(string); ok {
								dryRun, _ := block.Input["dryRun"].(bool)

								result := executeSed(filePath, searchPattern, replacePattern, dryRun, 10*time.Second)
								resultJSON, _ := json.Marshal(result)

								messages = append(messages, Message{
									Role: "user",
									Content: []ToolResult{
										{
											Type:      "tool_result",
											ToolUseID: block.ID,
											Content:   string(resultJSON),
										},
									},
								})
							}
						}
					}
				case "todo":
					if action, ok := block.Input["action"].(string); ok {
						filePath, _ := block.Input["filePath"].(string)
						if filePath == "" {
							filePath = "todo.md"
						}
						content, _ := block.Input["content"].(string)

						result := executeTodo(action, filePath, content, 10*time.Second)
						resultJSON, _ := json.Marshal(result)

						messages = append(messages, Message{
							Role: "user",
							Content: []ToolResult{
								{
									Type:      "tool_result",
									ToolUseID: block.ID,
									Content:   string(resultJSON),
								},
							},
						})
					}
				case "comby":
					if matchTemplate, ok := block.Input["matchTemplate"].(string); ok {
						rewriteTemplate, _ := block.Input["rewriteTemplate"].(string)
						target, _ := block.Input["target"].(string)
						matchOnly, _ := block.Input["matchOnly"].(bool)
						inPlace, _ := block.Input["inPlace"].(bool)
						diff, _ := block.Input["diff"].(bool)
						language, _ := block.Input["language"].(string)
						rule, _ := block.Input["rule"].(string)

						result := executeComby(matchTemplate, rewriteTemplate, target, matchOnly, inPlace, diff, language, rule, 10*time.Second)
						resultJSON, _ := json.Marshal(result)

						messages = append(messages, Message{
							Role: "user",
							Content: []ToolResult{
								{
									Type:      "tool_result",
									ToolUseID: block.ID,
									Content:   string(resultJSON),
								},
							},
						})
					}
				case "gofmt":
					target, _ := block.Input["target"].(string)
					listFiles, _ := block.Input["listFiles"].(bool)
					diff, _ := block.Input["diff"].(bool)
					write, _ := block.Input["write"].(bool)

					result := executeGofmt(target, listFiles, diff, write, 10*time.Second)
					resultJSON, _ := json.Marshal(result)

					messages = append(messages, Message{
						Role: "user",
						Content: []ToolResult{
							{
								Type:      "tool_result",
								ToolUseID: block.ID,
								Content:   string(resultJSON),
							},
						},
					})
				case "ask":
					if question, ok := block.Input["question"].(string); ok {
						result := executeAsk(question)
						resultJSON, _ := json.Marshal(result)

						messages = append(messages, Message{
							Role: "user",
							Content: []ToolResult{
								{
									Type:      "tool_result",
									ToolUseID: block.ID,
									Content:   string(resultJSON),
								},
							},
						})
					}
				case "finished":
					fmt.Println("\n✅ Task completed!")
					return nil
				}
			}
		}

		if !hasToolUse {
			messages = append(messages, Message{
				Role:    "user",
				Content: "Call a tool to continue the task",
			})
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Error: Provide a prompt as the first argument")
		os.Exit(1)
	}

	prompt := strings.Join(os.Args[1:], " ")

	fmt.Printf("\nStarting agent with prompt: \"%s\"\n", prompt)

	if err := runAgentLoop(prompt); err != nil {
		log.Fatal("Error:", err)
	}
}
