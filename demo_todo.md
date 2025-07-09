# Agent Todo Tool Demo

This file demonstrates the todo tool functionality.

## Example Usage

The todo tool supports the following actions:

### 1. Read a todo file
```
todo: {"action": "read", "filePath": "todo.md"}
```

### 2. Write a complete todo file
```
todo: {"action": "write", "filePath": "todo.md", "content": "# My Tasks\n\n- [ ] Task 1\n- [ ] Task 2"}
```

### 3. Add a new todo item
```
todo: {"action": "add", "filePath": "todo.md", "content": "Review code changes"}
```

### 4. Complete a todo item
```
todo: {"action": "complete", "filePath": "todo.md", "content": "Review code changes"}
```

### 5. Update a todo item
```
todo: {"action": "update", "filePath": "todo.md", "content": "Review code changes -> Review and test code changes"}
```

## Sample Todo List

- [x] Design todo tool schema
- [x] Implement todo functions
- [x] Add todo tool to agent
- [x] Create comprehensive tests
- [ ] Document usage examples
- [ ] Add error handling improvements

## Benefits

- **Planning**: Agent can create structured plans for multi-step tasks
- **Tracking**: Progress can be tracked and updated as work progresses
- **Persistence**: Todo lists persist across agent sessions
- **Flexibility**: Support for all standard todo operations (add, complete, update)
- **Markdown format**: Human-readable and compatible with existing tools