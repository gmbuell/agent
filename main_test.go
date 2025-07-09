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