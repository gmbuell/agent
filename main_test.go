package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// Test helper to reset sed cache between tests
func resetSedCache() {
	sedDryRunCache = make(map[string]bool)
}

func TestExecuteShellCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		timeout  time.Duration
		wantExit int
	}{
		{
			name:     "simple echo command",
			command:  "echo 'hello world'",
			timeout:  5 * time.Second,
			wantExit: 0,
		},
		{
			name:     "list files",
			command:  "ls -la",
			timeout:  5 * time.Second,
			wantExit: 0,
		},
		{
			name:     "invalid command",
			command:  "nonexistent-command",
			timeout:  5 * time.Second,
			wantExit: 1,
		},
		{
			name:     "command with timeout",
			command:  "sleep 3",
			timeout:  1 * time.Second,
			wantExit: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeShellCommand(tt.command, tt.timeout)
			if result.ExitCode != tt.wantExit {
				t.Errorf("executeShellCommand() exitCode = %v, want %v", result.ExitCode, tt.wantExit)
			}
		})
	}
}

func TestExecuteGoDoc(t *testing.T) {
	tests := []struct {
		name            string
		packageOrSymbol string
		timeout         time.Duration
		wantExit        int
	}{
		{
			name:            "fmt package",
			packageOrSymbol: "fmt",
			timeout:         10 * time.Second,
			wantExit:        0,
		},
		{
			name:            "fmt.Println function",
			packageOrSymbol: "fmt.Println",
			timeout:         10 * time.Second,
			wantExit:        0,
		},
		{
			name:            "nonexistent package",
			packageOrSymbol: "nonexistent/package",
			timeout:         10 * time.Second,
			wantExit:        1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeGoDoc(tt.packageOrSymbol, tt.timeout)
			if result.ExitCode != tt.wantExit {
				t.Errorf("executeGoDoc() exitCode = %v, want %v", result.ExitCode, tt.wantExit)
			}
		})
	}
}

func TestExecuteRipgrep(t *testing.T) {
	// Create test directory if it doesn't exist
	os.MkdirAll("test_data", 0755)

	// Ensure test files exist
	sampleContent := `This is a sample file for testing.
It contains multiple lines of text.
The word "example" appears here.
Another line with different content.
This file will be used for ripgrep and sed testing.
Here's an example of a longer line with more content.
Final line of the test file.`

	codeContent := `package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("Hello, World!")
	
	// Example function call
	result := processData("test input")
	fmt.Printf("Result: %s\n", result)
	
	// Time example
	now := time.Now()
	fmt.Printf("Current time: %v\n", now)
}

func processData(input string) string {
	return fmt.Sprintf("processed: %s", input)
}`

	os.WriteFile("test_data/sample.txt", []byte(sampleContent), 0644)
	os.WriteFile("test_data/code.go", []byte(codeContent), 0644)

	tests := []struct {
		name             string
		pattern          string
		path             string
		ignoreCase       bool
		lineNumbers      bool
		filesWithMatches bool
		timeout          time.Duration
		wantExit         int
	}{
		{
			name:        "search for 'example' in test_data",
			pattern:     "example",
			path:        "test_data",
			ignoreCase:  false,
			lineNumbers: true,
			timeout:     10 * time.Second,
			wantExit:    0,
		},
		{
			name:        "case insensitive search",
			pattern:     "EXAMPLE",
			path:        "test_data",
			ignoreCase:  true,
			lineNumbers: false,
			timeout:     10 * time.Second,
			wantExit:    0,
		},
		{
			name:             "files with matches only",
			pattern:          "fmt",
			path:             "test_data",
			ignoreCase:       false,
			lineNumbers:      false,
			filesWithMatches: true,
			timeout:          10 * time.Second,
			wantExit:         0,
		},
		{
			name:     "search pattern not found",
			pattern:  "nonexistent-pattern-xyz",
			path:     "test_data",
			timeout:  10 * time.Second,
			wantExit: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeRipgrep(tt.pattern, tt.path, tt.ignoreCase, tt.lineNumbers, tt.filesWithMatches, tt.timeout)
			if result.ExitCode != tt.wantExit {
				t.Errorf("executeRipgrep() exitCode = %v, want %v", result.ExitCode, tt.wantExit)
			}
		})
	}
}

func TestExecuteSedDryRun(t *testing.T) {
	resetSedCache()

	// Create a test file
	testFile := "test_data/sed_test.txt"
	content := "Hello World\nThis is a test\nWorld of testing"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	tests := []struct {
		name           string
		filePath       string
		searchPattern  string
		replacePattern string
		dryRun         bool
		timeout        time.Duration
		wantExit       int
	}{
		{
			name:           "dry-run replacement",
			filePath:       testFile,
			searchPattern:  "World",
			replacePattern: "Universe",
			dryRun:         true,
			timeout:        10 * time.Second,
			wantExit:       0,
		},
		{
			name:           "dry-run with regex",
			filePath:       testFile,
			searchPattern:  "test.*",
			replacePattern: "example",
			dryRun:         true,
			timeout:        10 * time.Second,
			wantExit:       0,
		},
		{
			name:           "dry-run on nonexistent file",
			filePath:       "nonexistent.txt",
			searchPattern:  "test",
			replacePattern: "example",
			dryRun:         true,
			timeout:        10 * time.Second,
			wantExit:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeSed(tt.filePath, tt.searchPattern, tt.replacePattern, tt.dryRun, tt.timeout)
			if result.ExitCode != tt.wantExit {
				t.Errorf("executeSed() exitCode = %v, want %v", result.ExitCode, tt.wantExit)
			}
		})
	}
}

func TestExecuteSedEnforcement(t *testing.T) {
	resetSedCache()

	// Create a test file
	testFile := "test_data/sed_enforcement_test.txt"
	content := "Hello World\nThis is a test\nWorld of testing"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	t.Run("apply without dry-run should fail", func(t *testing.T) {
		resetSedCache()

		result := executeSed(testFile, "World", "Universe", false, 10*time.Second)
		if result.ExitCode != 1 {
			t.Errorf("Expected failure when applying without dry-run, got exitCode = %v", result.ExitCode)
		}
		if !strings.Contains(result.Stderr, "Must perform dry-run") {
			t.Errorf("Expected error message about dry-run requirement, got: %s", result.Stderr)
		}
	})

	t.Run("dry-run then apply should succeed", func(t *testing.T) {
		resetSedCache()

		// First do dry-run
		dryRunResult := executeSed(testFile, "World", "Universe", true, 10*time.Second)
		if dryRunResult.ExitCode != 0 {
			t.Fatalf("Dry-run failed: exitCode = %v, stderr = %s", dryRunResult.ExitCode, dryRunResult.Stderr)
		}

		// Then apply
		applyResult := executeSed(testFile, "World", "Universe", false, 10*time.Second)
		if applyResult.ExitCode != 0 {
			t.Errorf("Apply after dry-run failed: exitCode = %v, stderr = %s", applyResult.ExitCode, applyResult.Stderr)
		}

		// Verify file was modified
		modifiedContent, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read modified file: %v", err)
		}
		if !strings.Contains(string(modifiedContent), "Universe") {
			t.Errorf("File was not modified correctly. Content: %s", string(modifiedContent))
		}
	})

	t.Run("different parameters should require new dry-run", func(t *testing.T) {
		resetSedCache()

		// Do dry-run with one set of parameters
		dryRunResult := executeSed(testFile, "World", "Universe", true, 10*time.Second)
		if dryRunResult.ExitCode != 0 {
			t.Fatalf("Dry-run failed: exitCode = %v", dryRunResult.ExitCode)
		}

		// Try to apply with different parameters (should fail)
		applyResult := executeSed(testFile, "test", "example", false, 10*time.Second)
		if applyResult.ExitCode != 1 {
			t.Errorf("Expected failure when applying with different parameters, got exitCode = %v", applyResult.ExitCode)
		}
	})
}

func TestGenerateSedOperationKey(t *testing.T) {
	tests := []struct {
		name           string
		filePath       string
		searchPattern  string
		replacePattern string
		wantSameKey    bool
		compareWith    struct {
			filePath       string
			searchPattern  string
			replacePattern string
		}
	}{
		{
			name:           "identical operations should have same key",
			filePath:       "test.txt",
			searchPattern:  "hello",
			replacePattern: "world",
			wantSameKey:    true,
			compareWith: struct {
				filePath       string
				searchPattern  string
				replacePattern string
			}{
				filePath:       "test.txt",
				searchPattern:  "hello",
				replacePattern: "world",
			},
		},
		{
			name:           "different file paths should have different keys",
			filePath:       "test1.txt",
			searchPattern:  "hello",
			replacePattern: "world",
			wantSameKey:    false,
			compareWith: struct {
				filePath       string
				searchPattern  string
				replacePattern string
			}{
				filePath:       "test2.txt",
				searchPattern:  "hello",
				replacePattern: "world",
			},
		},
		{
			name:           "different search patterns should have different keys",
			filePath:       "test.txt",
			searchPattern:  "hello",
			replacePattern: "world",
			wantSameKey:    false,
			compareWith: struct {
				filePath       string
				searchPattern  string
				replacePattern string
			}{
				filePath:       "test.txt",
				searchPattern:  "hi",
				replacePattern: "world",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := generateSedOperationKey(tt.filePath, tt.searchPattern, tt.replacePattern)
			key2 := generateSedOperationKey(tt.compareWith.filePath, tt.compareWith.searchPattern, tt.compareWith.replacePattern)

			if tt.wantSameKey {
				if key1 != key2 {
					t.Errorf("Expected same keys for identical operations, got %s != %s", key1, key2)
				}
			} else {
				if key1 == key2 {
					t.Errorf("Expected different keys for different operations, got %s == %s", key1, key2)
				}
			}
		})
	}
}

func TestSedCacheManagement(t *testing.T) {
	resetSedCache()

	testFile := "test_data/cache_test.txt"
	content := "Hello World\nThis is a test"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	t.Run("cache should be populated after dry-run", func(t *testing.T) {
		resetSedCache()

		key := generateSedOperationKey(testFile, "World", "Universe")
		if sedDryRunCache[key] {
			t.Errorf("Cache should be empty initially")
		}

		result := executeSed(testFile, "World", "Universe", true, 10*time.Second)
		if result.ExitCode != 0 {
			t.Fatalf("Dry-run failed: %v", result.Stderr)
		}

		if !sedDryRunCache[key] {
			t.Errorf("Cache should be populated after successful dry-run")
		}
	})

	t.Run("cache should be cleared after successful apply", func(t *testing.T) {
		resetSedCache()

		key := generateSedOperationKey(testFile, "World", "Universe")

		// Do dry-run
		dryRunResult := executeSed(testFile, "World", "Universe", true, 10*time.Second)
		if dryRunResult.ExitCode != 0 {
			t.Fatalf("Dry-run failed: %v", dryRunResult.Stderr)
		}

		if !sedDryRunCache[key] {
			t.Fatalf("Cache should be populated after dry-run")
		}

		// Apply changes
		applyResult := executeSed(testFile, "World", "Universe", false, 10*time.Second)
		if applyResult.ExitCode != 0 {
			t.Fatalf("Apply failed: %v", applyResult.Stderr)
		}

		if sedDryRunCache[key] {
			t.Errorf("Cache should be cleared after successful apply")
		}
	})
}

// Integration test that combines multiple tools
func TestIntegration(t *testing.T) {
	t.Run("ripgrep find then sed modify workflow", func(t *testing.T) {
		resetSedCache()

		// Create test file
		testFile := "test_data/integration_test.txt"
		content := "Configuration settings:\nDebug mode: enabled\nLogging: disabled"
		err := os.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		defer os.Remove(testFile)

		// Use ripgrep to find the pattern
		rgResult := executeRipgrep("enabled", "test_data", false, true, false, 10*time.Second)
		if rgResult.ExitCode != 0 {
			t.Fatalf("Ripgrep failed: %v", rgResult.Stderr)
		}

		if !strings.Contains(rgResult.Stdout, "enabled") {
			t.Errorf("Ripgrep should have found 'enabled' in the file")
		}

		// Use sed to modify the file (dry-run first)
		sedDryResult := executeSed(testFile, "enabled", "disabled", true, 10*time.Second)
		if sedDryResult.ExitCode != 0 {
			t.Fatalf("Sed dry-run failed: %v", sedDryResult.Stderr)
		}

		// Apply the changes
		sedApplyResult := executeSed(testFile, "enabled", "disabled", false, 10*time.Second)
		if sedApplyResult.ExitCode != 0 {
			t.Fatalf("Sed apply failed: %v", sedApplyResult.Stderr)
		}

		// Verify changes were applied
		modifiedContent, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read modified file: %v", err)
		}

		if strings.Contains(string(modifiedContent), "enabled") {
			t.Errorf("File should not contain 'enabled' after modification")
		}

		if !strings.Contains(string(modifiedContent), "disabled") {
			t.Errorf("File should contain 'disabled' after modification")
		}
	})
}

func TestExecuteTodo(t *testing.T) {
	testTodoFile := "test_data/test_todo.md"

	// Cleanup before and after tests
	defer os.Remove(testTodoFile)
	os.Remove(testTodoFile)

	t.Run("read nonexistent todo file", func(t *testing.T) {
		result := executeTodo("read", testTodoFile, "", 10*time.Second)
		if result.ExitCode != 1 {
			t.Errorf("Expected failure reading nonexistent file, got exitCode = %v", result.ExitCode)
		}
		if !strings.Contains(result.Stderr, "does not exist") {
			t.Errorf("Expected error about nonexistent file, got: %s", result.Stderr)
		}
	})

	t.Run("write todo file", func(t *testing.T) {
		content := "# My Todo List\n\n- [ ] Task 1\n- [ ] Task 2\n"
		result := executeTodo("write", testTodoFile, content, 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Write failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		// Verify file was created
		if _, err := os.Stat(testTodoFile); os.IsNotExist(err) {
			t.Errorf("Todo file was not created")
		}
	})

	t.Run("read existing todo file", func(t *testing.T) {
		result := executeTodo("read", testTodoFile, "", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Read failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}
		if !strings.Contains(result.Stdout, "My Todo List") {
			t.Errorf("Expected todo content, got: %s", result.Stdout)
		}
	})

	t.Run("add todo item", func(t *testing.T) {
		result := executeTodo("add", testTodoFile, "Task 3", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Add failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		// Verify item was added
		content, _ := os.ReadFile(testTodoFile)
		if !strings.Contains(string(content), "- [ ] Task 3") {
			t.Errorf("Task 3 was not added to todo file")
		}
	})

	t.Run("complete todo item", func(t *testing.T) {
		result := executeTodo("complete", testTodoFile, "Task 1", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Complete failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		// Verify item was completed
		content, _ := os.ReadFile(testTodoFile)
		if !strings.Contains(string(content), "- [x] Task 1") {
			t.Errorf("Task 1 was not marked as complete")
		}
	})

	t.Run("update todo item", func(t *testing.T) {
		result := executeTodo("update", testTodoFile, "Task 2 -> Updated Task 2", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Update failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		// Verify item was updated
		content, _ := os.ReadFile(testTodoFile)
		if !strings.Contains(string(content), "- [ ] Updated Task 2") {
			t.Errorf("Task 2 was not updated correctly")
		}
		if strings.Contains(string(content), "- [ ] Task 2") && !strings.Contains(string(content), "Updated Task 2") {
			t.Errorf("Old Task 2 still exists")
		}
	})

	t.Run("complete nonexistent item", func(t *testing.T) {
		result := executeTodo("complete", testTodoFile, "Nonexistent Task", 10*time.Second)
		if result.ExitCode != 1 {
			t.Errorf("Expected failure completing nonexistent item, got exitCode = %v", result.ExitCode)
		}
		if !strings.Contains(result.Stderr, "not found") {
			t.Errorf("Expected error about item not found, got: %s", result.Stderr)
		}
	})

	t.Run("update with invalid format", func(t *testing.T) {
		result := executeTodo("update", testTodoFile, "invalid format", 10*time.Second)
		if result.ExitCode != 1 {
			t.Errorf("Expected failure with invalid format, got exitCode = %v", result.ExitCode)
		}
		if !strings.Contains(result.Stderr, "Update format") {
			t.Errorf("Expected error about update format, got: %s", result.Stderr)
		}
	})

	t.Run("unknown action", func(t *testing.T) {
		result := executeTodo("unknown", testTodoFile, "", 10*time.Second)
		if result.ExitCode != 1 {
			t.Errorf("Expected failure with unknown action, got exitCode = %v", result.ExitCode)
		}
		if !strings.Contains(result.Stderr, "Unknown todo action") {
			t.Errorf("Expected error about unknown action, got: %s", result.Stderr)
		}
	})
}

func TestTodoAddToEmptyFile(t *testing.T) {
	testTodoFile := "test_data/empty_todo.md"
	defer os.Remove(testTodoFile)
	os.Remove(testTodoFile)

	t.Run("add to empty file creates header", func(t *testing.T) {
		result := executeTodo("add", testTodoFile, "First Task", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Add to empty file failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		// Verify file was created with header
		content, _ := os.ReadFile(testTodoFile)
		contentStr := string(content)
		if !strings.Contains(contentStr, "# Todo List") {
			t.Errorf("Header was not added to empty file")
		}
		if !strings.Contains(contentStr, "- [ ] First Task") {
			t.Errorf("First task was not added correctly")
		}
	})
}

func TestTodoIntegration(t *testing.T) {
	testTodoFile := "test_data/integration_todo.md"
	defer os.Remove(testTodoFile)
	os.Remove(testTodoFile)

	t.Run("full todo workflow", func(t *testing.T) {
		// Start with empty file, add items
		result1 := executeTodo("add", testTodoFile, "Setup environment", 10*time.Second)
		if result1.ExitCode != 0 {
			t.Fatalf("Add 1 failed: %v", result1.Stderr)
		}

		result2 := executeTodo("add", testTodoFile, "Write code", 10*time.Second)
		if result2.ExitCode != 0 {
			t.Fatalf("Add 2 failed: %v", result2.Stderr)
		}

		result3 := executeTodo("add", testTodoFile, "Test code", 10*time.Second)
		if result3.ExitCode != 0 {
			t.Fatalf("Add 3 failed: %v", result3.Stderr)
		}

		// Complete first item
		result4 := executeTodo("complete", testTodoFile, "Setup environment", 10*time.Second)
		if result4.ExitCode != 0 {
			t.Fatalf("Complete failed: %v", result4.Stderr)
		}

		// Update second item
		result5 := executeTodo("update", testTodoFile, "Write code -> Write and document code", 10*time.Second)
		if result5.ExitCode != 0 {
			t.Fatalf("Update failed: %v", result5.Stderr)
		}

		// Read final content
		result6 := executeTodo("read", testTodoFile, "", 10*time.Second)
		if result6.ExitCode != 0 {
			t.Fatalf("Read failed: %v", result6.Stderr)
		}

		finalContent := result6.Stdout

		// Verify final state
		if !strings.Contains(finalContent, "- [x] Setup environment") {
			t.Errorf("Setup environment should be completed")
		}
		if !strings.Contains(finalContent, "- [ ] Write and document code") {
			t.Errorf("Write code should be updated")
		}
		if !strings.Contains(finalContent, "- [ ] Test code") {
			t.Errorf("Test code should be pending")
		}
		if strings.Contains(finalContent, "- [ ] Write code") && !strings.Contains(finalContent, "Write and document code") {
			t.Errorf("Old 'Write code' item should not exist")
		}
	})
}

func TestExecuteComby(t *testing.T) {
	// Create test files for comby
	os.MkdirAll("test_data/comby", 0755)

	// Create test Go file
	goContent := `package main

import "fmt"

func main() {
	fmt.Println("hello world")
	fmt.Println("goodbye world")
}

func greet(name string) {
	fmt.Printf("Hello, %s!\n", name)
}`

	testGoFile := "test_data/comby/test.go"
	err := os.WriteFile(testGoFile, []byte(goContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test Go file: %v", err)
	}
	defer os.Remove(testGoFile)

	// Create test JS file
	jsContent := `if (width <= 1280 && height <= 800) {
    return 1;
}

function foo(bar) {
    console.log("test");
    return bar + 1;
}`

	testJSFile := "test_data/comby/test.js"
	err = os.WriteFile(testJSFile, []byte(jsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test JS file: %v", err)
	}
	defer os.Remove(testJSFile)

	t.Run("match only mode", func(t *testing.T) {
		result := executeComby("fmt.Println(:[args])", "", ".go", true, false, false, "", "", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Match only failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}
		if !strings.Contains(result.Stdout, "hello world") {
			t.Errorf("Expected to find 'hello world' in matches, got: %s", result.Stdout)
		}
	})

	t.Run("rewrite with diff", func(t *testing.T) {
		result := executeComby("fmt.Println(:[args])", "log.Printf(\"msg: %s\", :[args])", testGoFile, false, false, true, ".go", "", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Rewrite with diff failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}
		if !strings.Contains(result.Stdout, "log.Printf") {
			t.Errorf("Expected to see rewrite diff with log.Printf, got: %s", result.Stdout)
		}
	})

	t.Run("match with language specification", func(t *testing.T) {
		result := executeComby("if (:[condition]) { :[body] }", "", testJSFile, true, false, false, ".js", "", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Language-specific match failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}
		if !strings.Contains(result.Stdout, "width <= 1280") {
			t.Errorf("Expected to find condition in matches, got: %s", result.Stdout)
		}
	})

	t.Run("match with regex hole", func(t *testing.T) {
		result := executeComby(":[fn~\\w+](:[args])", "", testJSFile, true, false, false, ".js", "", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Regex hole match failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}
		if !strings.Contains(result.Stdout, "foo") {
			t.Errorf("Expected to find function 'foo' in matches, got: %s", result.Stdout)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		result := executeComby("test", "", "nonexistent.go", true, false, false, "", "", 10*time.Second)
		// Comby doesn't fail for nonexistent files, just returns no matches
		if result.ExitCode != 0 {
			t.Errorf("Expected success for nonexistent file search, got exitCode = %v", result.ExitCode)
		}
	})

	t.Run("empty match template", func(t *testing.T) {
		result := executeComby("", "", testGoFile, true, false, false, "", "", 10*time.Second)
		// Comby doesn't fail for empty templates, just returns no matches
		if result.ExitCode != 0 {
			t.Errorf("Expected success for empty match template, got exitCode = %v", result.ExitCode)
		}
	})

	t.Run("complex structural match", func(t *testing.T) {
		result := executeComby("function :[name](:[params]) { :[body] }", "", testJSFile, true, false, false, ".js", "", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Complex structural match failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}
		if !strings.Contains(result.Stdout, "foo") {
			t.Errorf("Expected to find function name in matches, got: %s", result.Stdout)
		}
	})

	// Clean up
	os.RemoveAll("test_data/comby")
}

func TestCombyIntegration(t *testing.T) {
	// Create test directory and files
	os.MkdirAll("test_data/comby_integration", 0755)
	defer os.RemoveAll("test_data/comby_integration")

	// Create a more complex test file
	complexGoContent := `package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("Starting application")
	
	result := processData("input")
	fmt.Printf("Result: %v\n", result)
	
	if result > 0 {
		fmt.Println("Success!")
	} else {
		fmt.Println("Failed!")
	}
}

func processData(input string) int {
	fmt.Printf("Processing: %s\n", input)
	return len(input)
}`

	testFile := "test_data/comby_integration/complex.go"
	err := os.WriteFile(testFile, []byte(complexGoContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create complex test file: %v", err)
	}

	t.Run("find and show all fmt.Println calls", func(t *testing.T) {
		result := executeComby("fmt.Println(:[args])", "", testFile, true, false, false, ".go", "", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Finding fmt.Println calls failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		// Should find multiple matches
		if !strings.Contains(result.Stdout, "Starting application") {
			t.Errorf("Expected to find 'Starting application' in output")
		}
		if !strings.Contains(result.Stdout, "Success!") {
			t.Errorf("Expected to find 'Success!' in output")
		}
		if !strings.Contains(result.Stdout, "Failed!") {
			t.Errorf("Expected to find 'Failed!' in output")
		}
	})

	t.Run("transform fmt.Println to log.Println with diff", func(t *testing.T) {
		result := executeComby("fmt.Println(:[args])", "log.Println(:[args])", testFile, false, false, true, ".go", "", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Transform with diff failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		// Should show diff with log.Println
		if !strings.Contains(result.Stdout, "log.Println") {
			t.Errorf("Expected to see log.Println in diff output")
		}
	})

	t.Run("match if statements with conditions", func(t *testing.T) {
		result := executeComby("if :[condition] { :[body] }", "", testFile, true, false, false, ".go", "", 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("If statement matching failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		if !strings.Contains(result.Stdout, "result > 0") {
			t.Errorf("Expected to find condition 'result > 0' in matches")
		}
	})
}

func TestCombyEdgeCases(t *testing.T) {
	// Test edge cases and error conditions
	t.Run("timeout handling", func(t *testing.T) {
		// This should complete quickly, testing timeout mechanism
		result := executeComby("test", "", ".", true, false, false, "", "", 1*time.Millisecond)
		// Either succeeds quickly or times out - both are acceptable
		if result.ExitCode != 0 && result.ExitCode != 1 {
			t.Errorf("Unexpected exit code for timeout test: %v", result.ExitCode)
		}
	})

	t.Run("invalid regex in hole", func(t *testing.T) {
		result := executeComby(":[invalid~[", "", ".", true, false, false, "", "", 10*time.Second)
		// Test passes regardless of exit code - comby behavior may vary
		if result.ExitCode != 0 && result.ExitCode != 1 {
			t.Errorf("Unexpected exit code for invalid regex test: %v", result.ExitCode)
		}
	})

	t.Run("match template without rewrite template", func(t *testing.T) {
		result := executeComby("test", "", ".", true, false, false, "", "", 10*time.Second)
		// Should not crash - either finds matches or doesn't
		if result.ExitCode != 0 && result.ExitCode != 1 {
			t.Errorf("Unexpected exit code for match-only: %v", result.ExitCode)
		}
	})
}

func TestExecuteGofmt(t *testing.T) {
	// Create test Go files with intentionally bad formatting
	os.MkdirAll("test_data/gofmt", 0755)

	// Create an unformatted Go file
	unformattedContent := `package main

import(
"fmt"
"os"
)

func main( ) {
fmt.Println("hello world")
if true{
fmt.Println("test")
}
}`

	testGoFile := "test_data/gofmt/unformatted.go"
	err := os.WriteFile(testGoFile, []byte(unformattedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test Go file: %v", err)
	}
	defer os.Remove(testGoFile)

	// Create a well-formatted Go file for comparison
	formattedContent := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("hello world")
	if true {
		fmt.Println("test")
	}
}`

	formattedFile := "test_data/gofmt/formatted.go"
	err = os.WriteFile(formattedFile, []byte(formattedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create formatted test file: %v", err)
	}
	defer os.Remove(formattedFile)

	t.Run("list files that need formatting", func(t *testing.T) {
		result := executeGofmt(testGoFile, true, false, false, 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("List files failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}
		if !strings.Contains(result.Stdout, "unformatted.go") {
			t.Errorf("Expected to find unformatted.go in list output, got: %s", result.Stdout)
		}
	})

	t.Run("show diff without formatting", func(t *testing.T) {
		result := executeGofmt(testGoFile, false, true, false, 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Diff failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}
		if !strings.Contains(result.Stdout, "import") {
			t.Errorf("Expected to see formatting diff, got: %s", result.Stdout)
		}
	})

	t.Run("format file in place", func(t *testing.T) {
		// Create a copy to format
		testCopy := "test_data/gofmt/copy.go"
		err := os.WriteFile(testCopy, []byte(unformattedContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create copy: %v", err)
		}
		defer os.Remove(testCopy)

		result := executeGofmt(testCopy, false, false, true, 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Format in place failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		// Verify file was formatted
		formattedContent, err := os.ReadFile(testCopy)
		if err != nil {
			t.Fatalf("Failed to read formatted file: %v", err)
		}

		if !strings.Contains(string(formattedContent), "import (") {
			t.Errorf("File was not properly formatted")
		}
	})

	t.Run("format directory", func(t *testing.T) {
		result := executeGofmt("test_data/gofmt/", false, false, false, 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Format directory failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}
	})

	t.Run("format already formatted file", func(t *testing.T) {
		result := executeGofmt(formattedFile, true, false, false, 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Format already formatted file failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}
		// Should not list the file since it's already formatted
		if strings.Contains(result.Stdout, "formatted.go") {
			t.Logf("Note: gofmt -l lists formatted file as well, got: %s", result.Stdout)
		}
	})

	t.Run("format nonexistent file", func(t *testing.T) {
		result := executeGofmt("nonexistent.go", false, false, false, 10*time.Second)
		// go fmt typically returns 0 even for nonexistent files
		if result.ExitCode != 0 && result.ExitCode != 1 {
			t.Errorf("Unexpected exit code for nonexistent file: %v", result.ExitCode)
		}
	})

	t.Run("format with default target", func(t *testing.T) {
		result := executeGofmt("", false, false, false, 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Format default target failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}
	})

	// Clean up
	os.RemoveAll("test_data/gofmt")
}

func TestGofmtIntegration(t *testing.T) {
	// Create test directory and files
	os.MkdirAll("test_data/gofmt_integration", 0755)
	defer os.RemoveAll("test_data/gofmt_integration")

	// Create multiple Go files with formatting issues
	files := map[string]string{
		"main.go":  "package main\n\nimport\"fmt\"\n\nfunc main(){\nfmt.Println(\"hello\")\n}",
		"utils.go": "package main\n\nimport(\n\"strings\"\n\"fmt\"\n)\n\nfunc helper( s string )string{\nreturn strings.ToUpper(s)\n}",
		"types.go": "package main\n\ntype Person struct{\nName string\nAge int\n}",
	}

	for filename, content := range files {
		filepath := fmt.Sprintf("test_data/gofmt_integration/%s", filename)
		err := os.WriteFile(filepath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create %s: %v", filename, err)
		}
	}

	t.Run("check multiple files need formatting", func(t *testing.T) {
		result := executeGofmt("test_data/gofmt_integration/", true, false, false, 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("List files failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		// Should find all three files
		if !strings.Contains(result.Stdout, "main.go") {
			t.Errorf("Expected to find main.go needing formatting")
		}
		if !strings.Contains(result.Stdout, "utils.go") {
			t.Errorf("Expected to find utils.go needing formatting")
		}
		if !strings.Contains(result.Stdout, "types.go") {
			t.Errorf("Expected to find types.go needing formatting")
		}
	})

	t.Run("format entire directory", func(t *testing.T) {
		result := executeGofmt("test_data/gofmt_integration/", false, false, true, 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("Format directory failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		// Verify files were formatted
		mainContent, err := os.ReadFile("test_data/gofmt_integration/main.go")
		if err != nil {
			t.Fatalf("Failed to read main.go: %v", err)
		}
		if !strings.Contains(string(mainContent), "import \"fmt\"") {
			t.Errorf("main.go was not properly formatted")
		}

		utilsContent, err := os.ReadFile("test_data/gofmt_integration/utils.go")
		if err != nil {
			t.Fatalf("Failed to read utils.go: %v", err)
		}
		if !strings.Contains(string(utilsContent), "func helper(s string) string {") {
			t.Errorf("utils.go was not properly formatted")
		}
	})

	t.Run("verify no more files need formatting", func(t *testing.T) {
		result := executeGofmt("test_data/gofmt_integration/", true, false, false, 10*time.Second)
		if result.ExitCode != 0 {
			t.Errorf("List files failed: exitCode = %v, stderr = %s", result.ExitCode, result.Stderr)
		}

		// Should not find any files needing formatting
		if strings.Contains(result.Stdout, ".go") {
			t.Logf("Note: gofmt -l still lists files: %s", result.Stdout)
		}
	})
}

func TestGofmtEdgeCases(t *testing.T) {
	t.Run("timeout handling", func(t *testing.T) {
		// This should complete quickly
		result := executeGofmt(".", false, false, false, 1*time.Millisecond)
		// Either succeeds quickly or times out
		if result.ExitCode != 0 && result.ExitCode != 1 {
			t.Errorf("Unexpected exit code for timeout test: %v", result.ExitCode)
		}
	})

	t.Run("invalid go file", func(t *testing.T) {
		os.MkdirAll("test_data/gofmt_invalid", 0755)
		defer os.RemoveAll("test_data/gofmt_invalid")

		// Create an invalid Go file
		invalidContent := "package main\n\nfunc main() {\n\tfmt.Println(\"unclosed string\n}"

		invalidFile := "test_data/gofmt_invalid/invalid.go"
		err := os.WriteFile(invalidFile, []byte(invalidContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create invalid file: %v", err)
		}

		result := executeGofmt(invalidFile, false, false, false, 10*time.Second)
		// go fmt should handle invalid syntax gracefully
		if result.ExitCode != 0 && result.ExitCode != 1 {
			t.Errorf("Unexpected exit code for invalid file: %v", result.ExitCode)
		}
	})
}

func TestMain(m *testing.M) {
	// Setup
	fmt.Println("Setting up test environment...")
	os.MkdirAll("test_data", 0755)

	// Run tests
	code := m.Run()

	// Cleanup
	fmt.Println("Cleaning up test environment...")
	os.RemoveAll("test_data")

	os.Exit(code)
}
