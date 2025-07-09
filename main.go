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
	defaultModel   = "claude-3-7-sonnet"
	maxTokens      = 200000
	maxInputTokens = 150000
	temperature    = 0.1
)

type ToolSchema struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  InputSchema `json:"parameters"`
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

// Message represents a chat message - used for both API requests and internal storage
type Message struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ChatMessage represents the OpenAI API format for messages
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
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
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	Tools       []ToolSchema  `json:"tools,omitempty"`
	ToolChoice  string        `json:"tool_choice,omitempty"`
}

type APIResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int           `json:"index"`
	Message      ChoiceMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type ChoiceMessage struct {
	Role      string     `json:"role"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ShellResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// Global state to track sed dry-run operations
var sedDryRunCache = make(map[string]bool)

// Global state for command execution - tracks which binaries are always allowed
var alwaysAllowedCommands = make(map[string]bool)

func promptForCommandExecution(command string) bool {
	// Skip prompting during tests (when TEST_MODE environment variable is set)
	if os.Getenv("TEST_MODE") == "true" {
		return true
	}

	// Extract the binary name from the command
	binary := strings.Fields(command)[0]
	if len(strings.Fields(command)) == 0 {
		return false
	}

	// Check if this binary is always allowed
	if alwaysAllowedCommands[binary] {
		return true
	}

	fmt.Printf("\n⚠️  About to execute shell command: %s\n", command)
	fmt.Printf("Options: (y)es, (n)o, (i)nstruct, (a)lways allow '%s': ", binary)

	var response string
	fmt.Scanln(&response)

	switch strings.ToLower(response) {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	case "i", "instruct":
		fmt.Print("Enter your instruction: ")
		var instruction string
		fmt.Scanln(&instruction)
		fmt.Printf("Instruction received: %s\n", instruction)
		return false // Don't execute, user provided instruction instead
	case "a", "always":
		alwaysAllowedCommands[binary] = true
		fmt.Printf("'%s' will always be allowed from now on.\n", binary)
		return true
	default:
		fmt.Println("Invalid response, defaulting to 'no'")
		return false
	}
}

func executeShellCommand(command string, timeout time.Duration) ShellResult {
	// Prompt user for confirmation before executing
	if !promptForCommandExecution(command) {
		return ShellResult{
			Stdout:   "",
			Stderr:   "Command execution cancelled by user",
			ExitCode: 1,
		}
	}

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

func convertMessagesToChat(messages []Message, systemPrompt string) []ChatMessage {
	var chatMessages []ChatMessage

	// Always add system prompt as first message
	chatMessages = append(chatMessages, ChatMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	fmt.Printf("🔧 Converting %d internal messages to chat format\n", len(messages))
	fmt.Printf("🔧 System prompt: %.100s...\n", systemPrompt)

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			if str, ok := msg.Content.(string); ok {
				chatMessages = append(chatMessages, ChatMessage{
					Role:    "user",
					Content: str,
				})
			} else if toolResults, ok := msg.Content.([]ToolResult); ok {
				// Handle tool results - for each tool result, create a tool message
				for _, result := range toolResults {
					chatMessages = append(chatMessages, ChatMessage{
						Role:       "tool",
						Content:    result.Content,
						ToolCallID: result.ToolUseID,
					})
				}
			}
		case "assistant":
			chatMsg := ChatMessage{Role: "assistant"}

			if blocks, ok := msg.Content.([]ContentBlock); ok {
				var textParts []string
				var toolCalls []ToolCall

				for _, block := range blocks {
					if block.Type == "text" {
						textParts = append(textParts, block.Text)
					} else if block.Type == "tool_use" {
						argsJSON, _ := json.Marshal(block.Input)
						toolCalls = append(toolCalls, ToolCall{
							ID:   block.ID,
							Type: "function",
							Function: ToolCallFunction{
								Name:      block.Name,
								Arguments: string(argsJSON),
							},
						})
					}
				}

				if len(textParts) > 0 {
					chatMsg.Content = strings.Join(textParts, "\n")
				}
				if len(toolCalls) > 0 {
					chatMsg.ToolCalls = toolCalls
				}
			}

			chatMessages = append(chatMessages, chatMsg)
		}
	}

	fmt.Printf("🔧 Generated %d chat messages for API\n", len(chatMessages))
	return chatMessages
}

func convertChoiceToContentBlocks(choice Choice) []ContentBlock {
	var blocks []ContentBlock

	// Add text content if present
	if choice.Message.Content != "" {
		blocks = append(blocks, ContentBlock{
			Type: "text",
			Text: choice.Message.Content,
		})
	}

	// Convert tool calls to ContentBlocks
	for _, toolCall := range choice.Message.ToolCalls {
		// Parse the arguments JSON
		var args map[string]interface{}
		json.Unmarshal([]byte(toolCall.Function.Arguments), &args)

		blocks = append(blocks, ContentBlock{
			Type:  "tool_use",
			ID:    toolCall.ID,
			Name:  toolCall.Function.Name,
			Input: args,
		})
	}

	return blocks
}

func callOpenAIAPI(request APIRequest) (*APIResponse, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Debug: Print request details
	fmt.Printf("🚀 API Request Details:\n")
	fmt.Printf("🚀 Model: %s\n", request.Model)
	fmt.Printf("🚀 Messages count: %d\n", len(request.Messages))
	fmt.Printf("🚀 Tools count: %d\n", len(request.Tools))
	if len(request.Messages) > 0 {
		fmt.Printf("🚀 First message role: %s\n", request.Messages[0].Role)
		if request.Messages[0].Role == "system" {
			fmt.Printf("🚀 System message: %.100s...\n", request.Messages[0].Content)
		}
	}

	// Optional: Print full request JSON for debugging
	if os.Getenv("DEBUG_API") == "true" {
		fmt.Printf("🔍 Full API Request JSON:\n%s\n", string(jsonData))
	}

	// Get API endpoint from environment variable, with default fallback
	apiURL := os.Getenv("OPENAI_API_URL")
	if apiURL == "" {
		apiURL = "https://api.openai.com/v1/chat/completions"
	}
	fmt.Printf("🚀 API URL: %s\n", apiURL)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

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

	// Debug: Print response details
	fmt.Printf("📥 API Response Status: %d\n", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("❌ API Error Response: %s\n", string(body))
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Optional: Print full response JSON for debugging
	if os.Getenv("DEBUG_API") == "true" {
		fmt.Printf("🔍 Full API Response JSON:\n%s\n", string(body))
	}

	var response APIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Debug: Print parsed response details
	fmt.Printf("📥 Response parsed successfully\n")
	fmt.Printf("📥 Choices count: %d\n", len(response.Choices))
	if len(response.Choices) > 0 {
		choice := response.Choices[0]
		fmt.Printf("📥 First choice role: %s\n", choice.Message.Role)
		if choice.Message.Content != "" {
			fmt.Printf("📥 Content length: %d chars\n", len(choice.Message.Content))
		}
		fmt.Printf("📥 Tool calls count: %d\n", len(choice.Message.ToolCalls))
	}

	return &response, nil
}

func createToolSchema(name, description string, parameters InputSchema) ToolSchema {
	return ToolSchema{
		Type: "function",
		Function: Function{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}

func runAgentLoop(initialPrompt string) error {
	shellCommandSchema := createToolSchema("shellCommand", "Execute a shell command and return the result", InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"command": {
				Type:        "string",
				Description: "The shell command to execute",
			},
		},
		Required: []string{"command"},
	})

	goDocSchema := createToolSchema("goDoc", "Execute go doc command to get documentation for Go packages, types, or functions", InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"packageOrSymbol": {
				Type:        "string",
				Description: "The package, type, or function to get documentation for (e.g., 'fmt', 'fmt.Println', 'net/http')",
			},
		},
		Required: []string{"packageOrSymbol"},
	})

	ripgrepSchema := createToolSchema("ripgrep", "Search for patterns in files using ripgrep (rg)", InputSchema{
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
	})

	sedSchema := createToolSchema("sed", "Search and replace text in files using sed. ENFORCED: You must ALWAYS do a dry-run (dryRun=true) first to show diff before applying changes (dryRun=false). The system will reject apply operations without a prior dry-run.", InputSchema{
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
	})

	todoSchema := createToolSchema("todo", "Manage todo.md files for planning and tracking multi-step changes", InputSchema{
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
	})

	combySchema := createToolSchema("comby", "Advanced structural search and replace tool for code. Comby uses template-based matching with holes (:[name]) to match code structurally, understanding balanced delimiters, comments, and strings. Examples: 'fmt.Println(:[args])' matches function calls, 'if (:[condition]) { :[body] }' matches if statements. Supports regex in holes with :[name~regex] syntax. Can match-only or rewrite code in-place with diff preview.", InputSchema{
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
	})

	gofmtSchema := createToolSchema("gofmt", "Format Go source code using 'go fmt'. IMPORTANT: Agent should run this regularly after creating or modifying Go files to maintain proper formatting. Use write=true to format files in-place.", InputSchema{
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
	})

	askSchema := createToolSchema("ask", "Ask the user for clarification or additional information when the agent needs input to proceed. Use this when requirements are unclear, multiple options exist, or user preferences are needed.", InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"question": {
				Type:        "string",
				Description: "The question or clarification request to present to the user. Be specific and clear about what information you need.",
			},
		},
		Required: []string{"question"},
	})

	finishedSchema := createToolSchema("finished", "Call this tool when the task is complete to end the conversation", InputSchema{
		Type:       "object",
		Properties: map[string]Property{},
		Required:   []string{},
	})

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

		// Get model from environment variable, with default fallback
		model := os.Getenv("OPENAI_MODEL")
		if model == "" {
			model = defaultModel
		}

		// Convert internal messages to OpenAI chat format
		chatMessages := convertMessagesToChat(messages, systemPrompt)

		temp := temperature
		topP := 1.0
		maxOut := maxTokens
		request := APIRequest{
			Model:       model,
			Messages:    chatMessages,
			MaxTokens:   &maxOut,
			Temperature: &temp,
			TopP:        &topP,
			Tools:       tools,
			ToolChoice:  "auto",
		}

		response, err := callOpenAIAPI(request)
		if err != nil {
			return fmt.Errorf("API call failed: %w", err)
		}

		// Extract content from OpenAI chat completions response format
		var contentBlocks []ContentBlock
		if len(response.Choices) > 0 {
			contentBlocks = convertChoiceToContentBlocks(response.Choices[0])
		}

		messages = append(messages, Message{
			Role:    "assistant",
			Content: contentBlocks,
		})

		hasToolUse := false

		for _, block := range contentBlocks {
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
