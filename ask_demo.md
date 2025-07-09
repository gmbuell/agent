# Ask Tool Demo

The ask tool allows the agent to request clarification from the user when needed.

## How it works

When the agent encounters uncertainty or needs user input, it can use the ask tool:

```json
{
  "question": "Which programming language would you like me to use for this project: Python, Go, or JavaScript?"
}
```

## Example Usage Scenarios

### 1. Multiple Options Available
```json
{
  "question": "I found multiple config files (config.json, config.yaml, config.toml). Which one should I modify?"
}
```

### 2. Unclear Requirements
```json
{
  "question": "You mentioned 'optimize the code' - should I focus on performance, memory usage, or code readability?"
}
```

### 3. Missing Information
```json
{
  "question": "What should be the maximum file size limit for the upload feature?"
}
```

### 4. Confirmation Needed
```json
{
  "question": "This will delete 15 test files. Are you sure you want to proceed? (yes/no)"
}
```

## Agent Workflow

1. Agent encounters ambiguity or needs input
2. Agent calls the ask tool with a specific question
3. User sees the question and provides a response
4. Agent receives the response and continues with the task

## Benefits

- **Interactive Clarification**: No more guessing what the user wants
- **Better Accuracy**: Agent can get specific requirements before proceeding
- **User Control**: User stays in the loop for important decisions
- **Flexible Guidance**: Agent can adapt based on user preferences

## Example Output

```
❓ Agent is asking for clarification:
Which database would you prefer for this project: SQLite, PostgreSQL, or MySQL?

👤 Your response: PostgreSQL

🤖 Great! I'll set up the project with PostgreSQL as the database.
```

The ask tool makes the agent more interactive and helps ensure tasks are completed according to user preferences and requirements.