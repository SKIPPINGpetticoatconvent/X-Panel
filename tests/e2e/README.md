# E2E Testing for X-Panel

This directory contains End-to-End (E2E) tests for X-Panel.

## Environment Configuration

E2E tests use a separate configuration to isolate test environment from user/production environment.

### Configuration Files

- **`.env.e2e`** (project root): Environment variables for E2E testing
- **`tests/e2e/config/e2e_config.go`**: Go configuration loader and helper functions

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GO_ENV` | `test` | Environment identifier |
| `XUI_DB_FOLDER` | `/tmp/x-panel-e2e/db` | Database directory |
| `XUI_LOG_FOLDER` | `/tmp/x-panel-e2e/logs` | Log directory |
| `XUI_LOG_LEVEL` | `debug` | Log level |
| `XUI_DEBUG` | `true` | Debug mode |
| `XUI_BIN_FOLDER` | `bin` | Binary files directory |
| `XUI_SNI_FOLDER` | `bin/sni` | SNI files directory |
| `XPANEL_RUN_IN_CONTAINER` | `true` | Container mode flag |
| `XUI_ENABLE_FAIL2BAN` | `false` | Fail2ban (disabled in tests) |
| `XUI_TEST_PORT` | `13688` | Test web panel port |
| `XUI_TEST_USERNAME` | `e2e_admin` | Test admin username |
| `XUI_TEST_PASSWORD` | `e2e_test_pass_123` | Test admin password |

## Running E2E Tests

### Prerequisites

1. Docker installed and running
2. Go 1.21 or later

### Quick Start

```bash
# Load E2E environment and run tests
source .env.e2e
go test -v ./tests/e2e/...

# Or use the test script
./scripts/run-e2e-tests.sh
```

### Custom Configuration

You can override any environment variable:

```bash
XUI_TEST_PORT=15000 go test -v ./tests/e2e/...
```

Or create a local override file `.env.e2e.local` (not tracked by git).

## Test Isolation

E2E tests are designed to be completely isolated from user environment:

1. **Separate database**: Uses `/tmp/x-panel-e2e/db` instead of `/etc/x-ui`
2. **Separate logs**: Uses `/tmp/x-panel-e2e/logs` instead of `/var/log`
3. **Different port**: Uses port 13688 instead of default panel port
4. **Test credentials**: Uses dedicated test username/password
5. **Disabled security features**: Fail2ban disabled to prevent test interference

## Cleanup

Test environment is automatically cleaned up after tests. To manually clean:

```bash
rm -rf /tmp/x-panel-e2e/