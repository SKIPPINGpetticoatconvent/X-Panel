# Priority instructions for file and shell operations

- For any operation that requires reading, writing, or executing on the local filesystem, the agent MUST use the MCP server named "DesktopCommander". Do not access local files directly via cloud API.
- Use MCP for:
  - File reads
  - File writes (via diffs)
  - Running shell commands
  - Persistent shells
  - Repository-level searches
  - Git operations
- Use the language model API only for content generation (code/text). Do not assume the model can access the local filesystem.
- If the DesktopCommander MCP server is unavailable, report that file/terminal operations are unavailable and request user instructions; do not attempt alternate hidden channels or change API keys.
- Follow least-privilege: only operate on directories explicitly mounted for the MCP instance.
