# GCP Vault Management API

A Go-based REST API service that manages GCP access tokens and service account keys through HashiCorp Vault's GCP secrets engine. Built with Gin framework and Viper for configuration management.

## Features

- **Automatic Vault GCP Secrets Engine Setup**: Configures and initializes Vault GCP secrets engine on startup
- **Roleset Management**: Create, list, and delete GCP rolesets with custom policies
- **Access Token Generation**: Generate short-lived GCP access tokens with configurable TTL
- **Service Account Key Management**: Create temporary service account keys with IAM bindings
- **Health Monitoring**: Built-in health checks for both the API and Vault connectivity
- **Comprehensive Logging**: Structured logging with request/response tracking
- **Graceful Shutdown**: Proper signal handling and graceful server shutdown
- **Configuration Management**: Flexible configuration via files and environment variables

## Prerequisites

- Go 1.21 or later
- HashiCorp Vault server (running and accessible)
- GCP project with appropriate permissions
- GCP service account with the following IAM roles:
  - `Service Account Admin`
  - `Service Account Token Creator`
  - `Project IAM Admin` (for IAM bindings)
  - `Security Admin` (if managing IAM policies)

## Quick Start

### 1. Setup Environment

```bash
# Clone or create the project
mkdir gcp-vault-api && cd gcp-vault-api

# Copy the provided files to your project directory
# - go.mod, main.go, and all package files
# - config.yaml and .env files

# Install dependencies
go mod download
```

### 2. Configure the Application

Create a `config.yaml` file or set environment variables:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

vault:
  address: "http://127.0.0.1:8200"
  token: "your-vault-token"
  namespace: ""  # Optional
  skip_verify: false

gcp:
  project_id: "your-gcp-project-id"
  service_account_path: "/path/to/service-account-key.json"
  default_token_scopes: "https://www.googleapis.com/auth/cloud-platform"
  default_ttl: "3600s"
  max_ttl: "7200s"
```

**Or using environment variables:**

```bash
export VAULT_ADDRESS="http://127.0.0.1:8200"
export VAULT_TOKEN="your-vault-token"
export GCP_PROJECT_ID="your-gcp-project-id"
export GCP_SERVICE_ACCOUNT_PATH="/path/to/service-account-key.json"
```

### 3. Start the Server

```bash
# Using make (recommended)
make run

# Or directly with go
go run .
```

The server will:
1. Load configuration
2. Connect to Vault
3. Initialize/configure the GCP secrets engine
4. Start the HTTP server on the configured port

## API Endpoints

### Health Check
```bash
GET /health
```

### Roleset Management

#### Create Roleset
```bash
POST /api/v1/rolesets/{name}
Content-Type: application/json

{
  "project": "your-gcp-project-id",
  "secret_type": "access_token|service_account_key",
  "token_scopes": "https://www.googleapis.com/auth/cloud-platform",
  "bindings": {
    "resource": "//cloudresourcemanager.googleapis.com/projects/your-project",
    "roles": ["roles/viewer"]
  },
  "ttl": "3600s",
  "max_ttl": "7200s"
}
```

#### List Rolesets
```bash
GET /api/v1/rolesets
```

#### Delete Roleset
```bash
DELETE /api/v1/rolesets/{name}
```

### Token Generation

#### Generate Access Token
```bash
POST /api/v1/tokens/{roleset-name}
Content-Type: application/json

{
  "ttl": "1800s"  # Optional
}
```

Response:
```json
{
  "message": "Access token generated successfully",
  "data": {
    "token": "ya29.c.c0ASRK0Ga...",
    "token_ttl": "29m59s",
    "expires_at_seconds": 1758020274
  }
}
```

### Service Account Keys

#### Generate Service Account Key
```bash
POST /api/v1/keys/{roleset-name}
```

Response:
```json
{
  "message": "Service account key generated successfully",
  "data": {
    "private_key_data": "base64-encoded-key-data",
    "key_algorithm": "KEY_ALG_RSA_2048",
    "key_type": "TYPE_GOOGLE_CREDENTIALS_FILE",
    "key_id": "key-id"
  }
}
```

## Development

### Using Make Commands

```bash
# Setup development environment
make setup

# Run in development mode (with hot reload if air is installed)
make dev

# Build the application
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Format and lint code
make fmt
make lint

# Clean build artifacts
make clean
```

### Project Structure

```
gcp-vault-api/
├── config/
│   └── config.go           # Configuration management
├── vault/
│   └── client.go           # Vault client and operations
├── handlers/
│   └── handlers.go         # HTTP handlers
├── main.go                 # Application entry point
├── config.yaml             # Configuration file
├── .env                    # Environment variables
├── Makefile               # Build and development tasks
└── README.md              # This file
```

## Usage Examples

### 1. Create an Access Token Roleset

```bash
curl -X POST "http://localhost:8080/api/v1/rolesets/my-token-roleset" \
  -H "Content-Type: application/json" \
  -d '{
    "project": "my-gcp-project",
    "secret_type": "access_token",
    "token_scopes": "https://www.googleapis.com/auth/cloud-platform",
    "ttl": "3600s"
  }'
```

### 2. Generate an Access Token

```bash
curl -X POST "http://localhost:8080/api/v1/tokens/my-token-roleset" \
  -H "Content-Type: application/json" \
  -d '{
    "ttl": "1800s"
  }'
```

### 3. Create a Service Account Key Roleset with IAM Bindings

```bash
curl -X POST "http://localhost:8080/api/v1/rolesets/my-sa-roleset" \
  -H "Content-Type: application/json" \
  -d '{
    "project": "my-gcp-project",
    "secret_type": "service_account_key",
    "bindings": {
      "resource": "//cloudresourcemanager.googleapis.com/projects/my-gcp-project",
      "roles": ["roles/viewer", "roles/storage.objectViewer"]
    }
  }'
```

### 4. Generate a Service Account Key

```bash
curl -X POST "http://localhost:8080/api/v1/keys/my-sa-roleset" \
  -H "Content-Type: application/json" \
  -d '{}'
```

## Configuration Options

### Server Configuration
- `SERVER_HOST`: Server bind address (default: "0.0.0.0")
- `SERVER_PORT`: Server port (default: 8080)

### Vault Configuration
- `VAULT_ADDRESS`: Vault server address (default: "http://127.0.0.1:8200")
- `VAULT_TOKEN`: Vault authentication token (required)
- `VAULT_NAMESPACE`: Vault namespace (optional)
- `VAULT_SKIP_VERIFY`: Skip TLS verification (default: false)

### GCP Configuration
- `GCP_PROJECT_ID`: GCP project ID (required)
- `GCP_SERVICE_ACCOUNT_PATH`: Path to service account JSON key file (required)
- `GCP_DEFAULT_TOKEN_SCOPES`: Default OAuth scopes for tokens
- `GCP_DEFAULT_TTL`: Default TTL for secrets (default: "3600s")
- `GCP_MAX_TTL`: Maximum TTL for secrets (default: "7200s")

## Security Considerations

1. **Vault Token**: Use a Vault token with minimal required permissions
2. **Service Account**: GCP service account should have only necessary IAM roles
3. **Network Security**: Run Vault and the API service in a secure network
4. **TLS**: Use TLS for production deployments
5. **Token Rotation**: Implement regular token rotation policies
6. **Logging**: Review logs regularly for suspicious activity

## Troubleshooting

### Common Issues

1. **"context deadline exceeded"**: Increase timeout or check network connectivity
2. **"GCP secrets engine not found"**: Ensure Vault has GCP secrets engine enabled
3. **"insufficient permissions"**: Check GCP service account IAM roles
4. **"vault sealed"**: Unseal Vault server before starting the API

### Debug Mode

Enable debug logging by setting the log level:

```bash
export LOG_LEVEL=debug
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test` and `make lint`
6. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
