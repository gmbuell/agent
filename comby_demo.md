# Comby Tool Demo

This file demonstrates the comby tool functionality for advanced structural code search and replace.

## What is Comby?

Comby is a tool for searching and rewriting code that understands language syntax, balanced delimiters, comments, and strings. It uses template-based matching with "holes" to capture and rewrite code structurally.

## Key Features

### 1. Template-based Matching with Holes
- Use `:[name]` to create holes that match code
- Holes understand language structure (balanced parentheses, brackets, braces)
- Holes respect comments and strings in the target language

### 2. Structural Understanding
- Matches balanced delimiters: `()`, `[]`, `{}`
- Respects language syntax (comments, strings, escape sequences)
- Context-aware matching based on surrounding code

### 3. Regex Support in Holes
- Use `:[name~regex]` to combine structural matching with regex
- Example: `:[fn~\w+](:[args])` matches function calls with word-character names

## Usage Examples

### Basic Function Call Transformation
```json
{
  "matchTemplate": "fmt.Println(:[args])",
  "rewriteTemplate": "log.Printf(\"msg: %s\", :[args])",
  "target": ".go",
  "matchOnly": false,
  "diff": true
}
```

### Finding All Function Definitions
```json
{
  "matchTemplate": "function :[name](:[params]) { :[body] }",
  "target": ".js",
  "matchOnly": true,
  "language": ".js"
}
```

### Complex Conditional Matching
```json
{
  "matchTemplate": "if (:[condition]) { :[body] }",
  "rewriteTemplate": "if (:[condition]) {\n  console.log(\"Condition checked\");\n  :[body]\n}",
  "target": "src/",
  "language": ".js",
  "diff": true
}
```

### Regex-enhanced Matching
```json
{
  "matchTemplate": ":[fn~\\w+](:[args~\\d+])",
  "rewriteTemplate": "enhanced_:[fn](:[args])",
  "target": ".go",
  "matchOnly": false
}
```

## Tool Parameters

### Required
- `matchTemplate`: The template to match code structure using holes

### Optional
- `rewriteTemplate`: Template for rewriting (required if not matchOnly)
- `target`: Files/directories/extensions to search (default: current directory)
- `matchOnly`: Only find matches, don't rewrite (default: false)
- `inPlace`: Modify files in place (default: false)
- `diff`: Show diff of changes (default: false)
- `language`: Force language matcher (.go, .js, .py, etc.)
- `rule`: Advanced rules for filtering matches

## Advanced Features

### Language-specific Matching
The tool automatically detects language from file extensions but you can force a specific matcher:
- `.go` for Go
- `.js` for JavaScript
- `.py` for Python
- `.java` for Java
- `.c` for C/C++
- `.generic` for generic matching

### Rules (Advanced)
Apply additional constraints to matches:
```json
{
  "rule": "where match.condition != \"true\""
}
```

## Benefits over Regular Expressions

1. **Structural Awareness**: Understands balanced delimiters
2. **Language Syntax**: Respects comments, strings, escape sequences
3. **Maintainability**: Templates are more readable than complex regex
4. **Precision**: Fewer false positives due to structural understanding
5. **Multi-language**: Same concepts work across different languages

## Example Workflow

1. **Find matches**: Use `matchOnly: true` to see what will be matched
2. **Preview changes**: Use `diff: true` to see proposed changes
3. **Apply changes**: Use `inPlace: true` to modify files (be careful!)

## Common Use Cases

- **Refactoring**: Change function calls, variable names, patterns
- **Code analysis**: Find specific patterns across large codebases
- **Migration**: Update API calls, import statements, syntax
- **Cleanup**: Remove or modify deprecated patterns
- **Documentation**: Extract function signatures, comments, etc.

The comby tool is particularly powerful for complex code transformations that would be difficult or error-prone with regular expressions alone.