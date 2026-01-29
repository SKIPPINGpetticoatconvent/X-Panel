---
name: xpanel-qa-check
description: Automated QA verification workflow for X-Panel (Go & Shell).
---

# X-Panel QA & Verification Skill

This skill enforces the project's quality assurance standards defined in `.agent/rules`. Use this skill before submitting any code changes.

## Verification Checklist

### 1. Go Code Verification

For any changes to `.go` files:

1.  **Format**:
    ```bash
    gofmt -w .
    # OR
    goimports -w .
    ```

2.  **Verify**:
    Run the mandatory verification chain:
    ```bash
    go build ./... && go test ./... && nilaway -test=false ./...
    ```
    > **Note**: `nilaway` must be installed (`go install go.uber.org/nilaway/cmd/nilaway@latest`).

### 2. Shell Script Verification

For any changes to `.sh` files:

1.  **Format**:
    ```bash
    shfmt -i 2 -w -s .
    ```

2.  **Lint**:
    ```bash
    shellcheck <script_name>.sh
    ```

### 3. TOML Configuration Verification

For any changes to `.toml` files:

1.  **Format & Check**:
    ```bash
    taplo fmt --check
    ```

## Automated QA Script

You can also run the project's master QA script if available:

```bash
./QA/run_qa.sh
```

## Troubleshooting

- **NilAway Errors**: If `nilaway` reports false positives, investigate the code path. Do not suppress errors without understanding.
- **Shellcheck Warnings**: Fix all warnings. Use `# shellcheck disable=SCxxxx` only if absolutely necessary and justified.
