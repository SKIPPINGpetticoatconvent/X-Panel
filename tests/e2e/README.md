# E2E Testing for X-Panel

This directory contains comprehensive End-to-End (E2E) tests for X-Panel, designed with a modular and maintainable architecture.

## Architecture Overview

The E2E testing framework is organized into several layers:

```
tests/e2e/
├── config/          # Configuration management
├── infra/           # Infrastructure management (Testcontainers)
├── api/             # API client encapsulation
├── scenarios/       # Test scenarios (modular test cases)
├── utils/           # Helper utilities
├── Dockerfile       # Test container definition
└── README.md        # This file
```

### Key Components

- **API Layer** (`api/`): Encapsulates all HTTP API calls with session management
- **Infrastructure Layer** (`infra/`): Manages Docker containers and test environment
- **Scenario Layer** (`scenarios/`): Modular test cases focused on specific features
- **Configuration Layer** (`config/`): Environment and test configuration management
- **Utilities** (`utils/`): Helper functions for test data generation

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

## Test Scenarios

### Core Test Suites

1. **Authentication Tests** (`auth_test.go`)
   - Valid login/logout flows
   - Invalid credential handling
   - Session management

2. **Inbound CRUD Tests** (`inbound_crud_test.go`)
   - Create, read, update, delete inbounds
   - Client management within inbounds
   - Traffic statistics

3. **Traffic & Connectivity Tests** (`traffic_test.go`)
   - Real traffic connectivity verification
   - Proxy functionality testing
   - Client container integration

4. **Backup & Restore Tests** (`backup_test.go`)
   - Database backup functionality
   - Data restoration verification
   - Service restart handling

5. **Error Handling Tests** (`error_handling_test.go`)
   - Invalid input validation
   - Resource conflict scenarios
   - Edge case handling

### Test Execution Model

- **TestMain**: Global setup/teardown in `main_test.go`
- **Container Lifecycle**: Single X-Panel container per test run
- **Data Isolation**: Each test scenario manages its own test data
- **Automatic Cleanup**: Test data cleaned up after each scenario

## Running E2E Tests

### Prerequisites

1. Docker installed and running
2. Go 1.21 or later

### Quick Start

```bash
# Load E2E environment and run all tests
source .env.e2e
go test -v ./tests/e2e/scenarios/...

# Run specific test scenario
go test -v ./tests/e2e/scenarios/ -run TestAuthentication

# Run tests in short mode (skip E2E)
go test -short ./tests/e2e/scenarios/...
```

### Custom Configuration

You can override any environment variable:

```bash
XUI_TEST_PORT=15000 go test -v ./tests/e2e/scenarios/...
```

Or create a local override file `.env.e2e.local` (not tracked by git).

## Test Isolation

E2E tests are designed to be completely isolated from user environment:

1. **Separate database**: Uses `/tmp/x-panel-e2e/db` instead of `/etc/x-ui`
2. **Separate logs**: Uses `/tmp/x-panel-e2e/logs` instead of `/var/log`
3. **Different port**: Uses port 13688 instead of default panel port
4. **Test credentials**: Uses dedicated test username/password
5. **Disabled security features**: Fail2ban disabled to prevent test interference

## Development Guidelines

### Adding New Test Scenarios

1. Create new test file in `scenarios/` directory
2. Follow naming convention: `{feature}_test.go`
3. Use existing API client and utilities
4. Implement proper cleanup in test functions
5. Add documentation for complex scenarios

### Extending API Client

Add new methods to `api/client.go` following existing patterns:
- Methods should handle authentication automatically
- Include proper error handling and logging
- Return structured data where possible

### Infrastructure Changes

Modify `infra/` components for new container requirements:
- Add new container types in separate files
- Implement proper cleanup in `Terminate` methods
- Document container dependencies and configurations

## Cleanup

Test environment is automatically cleaned up after tests. To manually clean:

```bash
rm -rf /tmp/x-panel-e2e/
```

## Troubleshooting

### Common Issues

1. **Container startup failures**: Check Docker daemon status
2. **Port conflicts**: Ensure port 13688 is available
3. **Test timeouts**: Increase timeout values in slow environments
4. **Database issues**: Check `/tmp/x-panel-e2e/` permissions

### Debug Mode

Enable detailed logging:

```bash
XUI_LOG_LEVEL=debug XUI_DEBUG=true go test -v ./tests/e2e/scenarios/...
```

## CI/CD Integration

Tests are designed to run in CI environments with Docker support. See `.github/workflows/qa-e2e-test.yml` for workflow configuration.