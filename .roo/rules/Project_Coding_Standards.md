# Project Coding Standards

## 1. Project Structure
- **`config/`**: Configuration loading and management.
- **`database/`**: Database interactions, models, and migrations.
- **`web/`**: Web server, controllers, and services.
    - **`service/`**: Business logic.
    - **`controller/`**: Request handling.
    - **`html/`**: Frontend templates.
- **`xray/`**: Core Xray integration logic.
- **`tests/`**: E2E and integration tests.

## 2. Go Coding Standards
### Naming Conventions
- Use **CamelCase** for internal variables/functions (e.g., `userCount`).
- Use **PascalCase** for exported variables/functions (e.g., `NewServer`).
- Interface names should usually end in `er` (e.g., `Reader`, `Writer`).

### Error Handling
- Always check errors immediately:
  ```go
  if err != nil {
      return fmt.Errorf("context: %w", err)
  }
  ```
- Use custom error types for domain-specific errors.

### Concurrency
- Use `context` to manage cancellation and timeouts.
- Avoid sharing memory; communicate by sharing memory (Channels).

## 3. Web/Frontend Standards
- Follow **Biome** configuration (`biome.json`) for formatting.
- HTML templates should be organized by component/page in `web/html/`.

## 4. Testing
- **Unit Tests**: Required for all new business logic (`_test.go`).
- **E2E Tests**: Critical paths must be covered in `tests/e2e/`.
- Run tests before pushing: `go test ./...`

## 5. Tooling & Workflow
- **Formatting**: Run `dprint fmt` or configured formatter before commit.
- **Commits**: Use conventional commits (e.g., `feat: add new panel`, `fix: resolve login bug`).