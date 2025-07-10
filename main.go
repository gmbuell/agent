package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type BashArgs struct {
	Command string `json:"command"`
}

type AgentState struct {
	client           *openai.Client
	allowedCommands  map[string]bool
	conversationMsgs []openai.ChatCompletionMessageParamUnion
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	baseURL := os.Getenv("OPENAI_BASE_URL")

	var client *openai.Client
	if baseURL != "" {
		c := openai.NewClient(
			option.WithAPIKey(apiKey),
			option.WithBaseURL(baseURL),
		)
		client = &c
	} else {
		c := openai.NewClient(
			option.WithAPIKey(apiKey),
		)
		client = &c
	}

	agent := &AgentState{
		client:          client,
		allowedCommands: make(map[string]bool),
	}

	for {
		var instruction string
		var shouldQuit bool

		err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Enter your instruction").
					Placeholder("What would you like me to do?").
					Value(&instruction),
				huh.NewConfirm().
					Title("Do you want to quit?").
					Value(&shouldQuit),
			),
		).Run()

		if err != nil {
			log.Printf("Error with form: %v", err)
			break
		}

		if shouldQuit {
			break
		}

		if strings.TrimSpace(instruction) == "" {
			continue
		}

		agent.conversationMsgs = []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage("You are a helpful assistant that MUST use tools to complete tasks. You have access to 'bash' tool for executing commands and 'finish' tool when the task is complete. You MUST call one of these tools in every response - never respond without using a tool."),
			openai.UserMessage(instruction),
		}

		agent.runAgentLoop()
	}
}

func (a *AgentState) runAgentLoop() {
	for {
		params := openai.ChatCompletionNewParams{
			Model:    "claude-3-7-sonnet",
			Messages: a.conversationMsgs,
			Tools: []openai.ChatCompletionToolParam{
				{
					Type: "function",
					Function: openai.FunctionDefinitionParam{
						Name:        "bash",
						Description: openai.String("Execute a shell command using bash"),
						Parameters: openai.FunctionParameters{
							"type": "object",
							"properties": map[string]interface{}{
								"command": map[string]interface{}{
									"type":        "string",
									"description": "The shell command to execute",
								},
							},
							"required": []string{"command"},
						},
					},
				},
				{
					Type: "function",
					Function: openai.FunctionDefinitionParam{
						Name:        "finish",
						Description: openai.String("Finish the current task and exit the agent loop"),
						Parameters: openai.FunctionParameters{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				},
			},
		}

		resp, err := a.callWithRetry(params)
		if err != nil {
			log.Printf("Error calling OpenAI API after retries: %v", err)
			return
		}

		if len(resp.Choices) == 0 {
			log.Println("No response from OpenAI")
			return
		}

		choice := resp.Choices[0]
		a.conversationMsgs = append(a.conversationMsgs, choice.Message.ToParam())

		if len(choice.Message.ToolCalls) == 0 {
			fmt.Printf("Agent: %s\n", choice.Message.Content)
			
			// Force the agent to use tools by adding a reminder message
			a.conversationMsgs = append(a.conversationMsgs, openai.UserMessage(
				"You must use either the 'bash' tool to execute commands or the 'finish' tool to complete the task. Please call one of the available tools.",
			))
			continue
		}

		for _, toolCall := range choice.Message.ToolCalls {
			switch toolCall.Function.Name {
			case "bash":
				var args BashArgs
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					log.Printf("Error parsing bash arguments: %v", err)
					continue
				}

				result := a.handleBashCommand(args.Command)
				a.conversationMsgs = append(a.conversationMsgs, openai.ToolMessage(result, toolCall.ID))

			case "finish":
				fmt.Println("Agent finished the task.")
				return

			default:
				log.Printf("Unknown tool: %s", toolCall.Function.Name)
			}
		}
	}
}

func (a *AgentState) handleBashCommand(command string) string {
	baseCommand := strings.Fields(command)[0]

	if !a.allowedCommands[baseCommand] {
		fmt.Printf("Agent wants to execute: %s\n", command)
		
		var choice string
		err := huh.NewSelect[string]().
			Title("Allow this command?").
			Options(
				huh.NewOption("Yes (allow once)", "yes"),
				huh.NewOption("No (deny)", "no"),
				huh.NewOption(fmt.Sprintf("Always allow '%s'", baseCommand), "always"),
				huh.NewOption("Provide alternative instructions", "instruct"),
			).
			Value(&choice).
			Run()
		
		if err != nil {
			return "Permission denied - selection error"
		}
		
		switch choice {
		case "yes":
			// Allow this one time
		case "no":
			return "Permission denied by user"
		case "always":
			a.allowedCommands[baseCommand] = true
			fmt.Printf("Command '%s' will always be allowed\n", baseCommand)
		case "instruct":
			var instruction string
			err := huh.NewInput().
				Title("Enter alternative instructions for the agent").
				Placeholder("What should the agent do instead?").
				Value(&instruction).
				Run()
			
			if err != nil || strings.TrimSpace(instruction) == "" {
				return "Permission denied - no alternative instructions provided"
			}
			
			a.conversationMsgs = append(a.conversationMsgs, openai.UserMessage(instruction))
			return "User provided alternative instructions"
		default:
			return "Permission denied - invalid response"
		}
	}

	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Command failed: %s\nOutput: %s", err.Error(), string(output))
	}

	return string(output)
}

func (a *AgentState) callWithRetry(params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	maxRetries := 10
	baseDelay := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := a.client.Chat.Completions.New(context.Background(), params)

		if err == nil {
			return resp, nil
		}

		// Check if it's a 500 error that we should retry
		if shouldRetry(err) && attempt < maxRetries {
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
			log.Printf("API call failed (attempt %d/%d): %v. Retrying in %v...", attempt+1, maxRetries+1, err, delay)
			time.Sleep(delay)
			continue
		}

		return nil, err
	}

	return nil, fmt.Errorf("exceeded maximum retries")
}

func shouldRetry(err error) bool {
	// Check if it's an HTTP 500 error
	if httpErr, ok := err.(*openai.Error); ok {
		return httpErr.StatusCode >= 500 && httpErr.StatusCode < 600
	}
	return false
}
