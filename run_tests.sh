#!/bin/bash

echo "🧪 Running comprehensive tests for Agent functionality..."
echo "=================================================="

# Build the agent first
echo "📦 Building agent..."
go build -o agent main.go
if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi

echo "✅ Build successful!"
echo

# Run tests
echo "🏃 Running tests..."
go test -v

# Check test results
if [ $? -eq 0 ]; then
    echo
    echo "🎉 All tests passed!"
    echo
    echo "📊 Test Coverage Summary:"
    echo "- ✅ Shell command execution (including timeouts)"
    echo "- ✅ Go doc functionality"
    echo "- ✅ Ripgrep search functionality"
    echo "- ✅ Sed dry-run enforcement mechanism"
    echo "- ✅ Sed dry-run to apply workflow"
    echo "- ✅ Operation key generation and caching"
    echo "- ✅ Integration tests"
    echo
    echo "🔧 Tools tested:"
    echo "- shellCommand: Execute shell commands with timeout"
    echo "- goDoc: Access Go documentation"
    echo "- ripgrep: Fast file search with various options"
    echo "- sed: Safe file editing with mandatory dry-run"
    echo "- todo: Manage todo.md files for planning and tracking"
    echo "- comby: Advanced structural code search and replace"
    echo "- gofmt: Format Go source code with various options"
    echo
    echo "🛡️ Security features verified:"
    echo "- Sed dry-run enforcement prevents accidental modifications"
    echo "- File existence validation"
    echo "- Timeout handling for all commands"
    echo "- Proper error handling and reporting"
else
    echo
    echo "❌ Some tests failed!"
    exit 1
fi