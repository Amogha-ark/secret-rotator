# Secret Rotator Operator

A Kubernetes operator that automatically synchronizes secrets from HashiCorp Vault to Kubernetes secrets, ensuring your applications always have access to the latest credentials without manual intervention.

[![Go Report Card](https://goreportcard.com/badge/github.com/Amogha-rao/secret-rotator-operator)](https://goreportcard.com/report/github.com/Amogha-rao/secret-rotator-operator)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

## 🎯 What It Does

The Secret Rotator Operator bridges the gap between HashiCorp Vault and Kubernetes by:

- **🔄 Automatic Synchronization**: Continuously monitors Vault secrets and updates corresponding Kubernetes secrets
- **⏰ Scheduled Polling**: Checks Vault every 10 minutes for secret changes (configurable)
- **🔍 Change Detection**: Only updates Kubernetes secrets when actual changes are detected in Vault
- **📊 Status Tracking**: Maintains rotation history and timestamps for audit purposes
- **🚀 Zero Downtime**: Updates secrets without affecting running applications
- **🔐 Secure**: Uses Vault's authentication mechanisms and Kubernetes RBAC

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Vault Server  │◄───│ Secret Rotator  │───►│ Kubernetes API  │
│                 │    │   Operator      │    │                 │
│  /secret/data/  │    │                 │    │ Secret Objects  │
│   myapp/db      │    │ Polls every     │    │                 │
└─────────────────┘    │  10 minutes     │    └─────────────────┘
                       └─────────────────┘
                               ▲
                               │
                       ┌─────────────────┐
                       │ SecretRotation  │
                       │ Custom Resource │
                       │                 │
                       │ vaultPath:      │
                       │ targetSecret:   │
                       └─────────────────┘
```

## 🚀 Quick Start

### Prerequisites

- **Kubernetes cluster** (v1.11.3+)
- **kubectl** configured to access your cluster
- **HashiCorp Vault** server (running and accessible)
- **Go** (v1.24.0+) for development

### 1. Install the Operator

**Install the CRDs:**
```bash
make install
```

**Run locally for development:**
```bash
# Set Vault environment variables
export VAULT_ADDR='http://127.0.0.1:8200'
export VAULT_TOKEN='your-vault-token'

# Run the operator
make run
```

**Or deploy to cluster:**
```bash
make docker-build docker-push IMG=<your-registry>/secret-rotator:latest
make deploy IMG=<your-registry>/secret-rotator:latest
```

### 2. Set Up Vault (Development)

```bash
# Start Vault in dev mode
vault server -dev

# Enable KV v2 secrets engine
vault secrets enable -path=secret kv-v2

# Add some test data
vault kv put secret/myapp/database \
  username=admin \
  password=secret123 \
  host=db.example.com
```

### 3. Create a SecretRotation Resource

```yaml
# config/samples/secrets_v1alpha1_secretrotation.yaml
apiVersion: secrets.github.com/v1alpha1
kind: SecretRotation
metadata:
  name: myapp-database-rotation
  namespace: default
spec:
  vaultPath: "secret/data/myapp/database"  # Vault KV v2 path
  targetSecret: "myapp-database-secret"    # Kubernetes secret name
```

```bash
kubectl apply -f config/samples/secrets_v1alpha1_secretrotation.yaml
```

### 4. Verify the Secret

```bash
# Check if the secret was created
kubectl get secret myapp-database-secret -o yaml

# Check the SecretRotation status
kubectl get secretrotation myapp-database-rotation -o yaml
```

## 📖 Usage Examples

### Basic Database Credentials

```yaml
apiVersion: secrets.github.com/v1alpha1
kind: SecretRotation
metadata:
  name: postgres-creds
spec:
  vaultPath: "secret/data/production/postgres"
  targetSecret: "postgres-credentials"
```

### Multiple Applications

```yaml
# Frontend API keys
---
apiVersion: secrets.github.com/v1alpha1
kind: SecretRotation
metadata:
  name: frontend-api-keys
spec:
  vaultPath: "secret/data/frontend/api-keys"
  targetSecret: "frontend-secrets"

# Backend service credentials  
---
apiVersion: secrets.github.com/v1alpha1
kind: SecretRotation
metadata:
  name: backend-service-creds
spec:
  vaultPath: "secret/data/backend/service-account"
  targetSecret: "backend-credentials"
```

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VAULT_ADDR` | Vault server address | `http://127.0.0.1:8200` |
| `VAULT_TOKEN` | Vault authentication token | None (required) |

### SecretRotation Spec

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `vaultPath` | string | Path to secret in Vault (include `/data/` for KV v2) | ✅ |
| `targetSecret` | string | Name of the Kubernetes secret to create/update | ✅ |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `lastRotation` | timestamp | Last time the secret was updated |

## ⚙️ How It Helps

### 🔄 Automated Secret Rotation
- **Problem**: Manual secret rotation is error-prone and time-consuming
- **Solution**: Automatically detects and applies secret changes from Vault

### 🔒 Enhanced Security
- **Problem**: Secrets become stale and may be compromised over time  
- **Solution**: Ensures applications always use fresh, rotated credentials

### 📈 Operational Efficiency
- **Problem**: DevOps teams spend time manually updating secrets across environments
- **Solution**: Set it once, let it run automatically with full audit trail

### 🚫 Zero Application Downtime
- **Problem**: Secret updates often require application restarts
- **Solution**: Updates secrets in-place, applications pick up changes naturally

### 🎯 GitOps Friendly
- **Problem**: Secrets in Git repositories pose security risks
- **Solution**: Secrets stay in Vault, only configuration goes in Git

## 🔍 Monitoring and Troubleshooting

### Check Operator Logs
```bash
# If running locally
make run

# If deployed to cluster
kubectl logs -f deployment/secret-rotator-controller-manager -n secret-rotator-system
```

### Check SecretRotation Status
```bash
kubectl describe secretrotation myapp-database-rotation
```

### Common Issues

**1. Vault Connection Failed**
```
Error: failed to read from Vault
```
- Verify `VAULT_ADDR` and `VAULT_TOKEN`
- Check network connectivity to Vault

**2. Permission Denied**
```
Error: failed to create Kubernetes Secret
```
- Verify RBAC permissions
- Check if the operator has `secrets` permissions in the target namespace

**3. Secret Not Found**
```
Vault secret not found or empty
```
- Verify the `vaultPath` is correct
- For KV v2, ensure path includes `/data/` (e.g., `secret/data/myapp/db`)

## 🛠️ Development

### Running Tests
```bash
make test
```

### Building and Deploying
```bash
# Build locally
make build

# Build and push Docker image
make docker-build docker-push IMG=<your-registry>/secret-rotator:tag

# Deploy to cluster
make deploy IMG=<your-registry>/secret-rotator:tag
```

### Project Structure
```
├── api/v1alpha1/           # CRD definitions
├── config/                 # Kubernetes manifests
├── internal/controller/    # Controller logic
├── cmd/                    # Main application entry point
└── test/                   # Test files
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📋 Roadmap

- [ ] Support for different Vault authentication methods (Kubernetes, AWS IAM)
- [ ] Configurable polling intervals via CRD spec
- [ ] Webhook-based immediate rotation triggers
- [ ] Support for selective key synchronization
- [ ] Metrics and Prometheus integration
- [ ] Helm chart for easy installation

## 📄 License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

---

**Made with ❤️ for the Kubernetes and HashiCorp Vault community**

