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

	finishedSchema := ToolSchema{
		Name:        "finished",
		Description: "Call this tool when the task is complete to end the conversation",
		InputSchema: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
			Required:   []string{},
		},
	}

	tools := []ToolSchema{shellCommandSchema, goDocSchema, ripgrepSchema, sedSchema, finishedSchema}

	systemPrompt := "You are an AI agent that can run shell commands, access Go documentation, search files, and edit text files to accomplish tasks.\n" +
		"Use the provided tools to complete the user's task:\n" +
		"- shellCommand: Execute any shell command\n" +
		"- goDoc: Get documentation for Go packages, types, or functions\n" +
		"- ripgrep: Search for patterns in files using ripgrep (fast file search)\n" +
		"- sed: Search and replace text in files. MANDATORY: You MUST do dry-run (dryRun=true) first, then apply (dryRun=false). System enforces this workflow.\n" +
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
