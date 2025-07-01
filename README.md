# Secret Rotator Operator

A Kubernetes operator that automatically synchronizes secrets from HashiCorp Vault to Kubernetes secrets and **triggers rolling updates** of your workloads when secrets change, ensuring your applications always have access to the latest credentials without manual intervention.

[![Go Report Card](https://goreportcard.com/badge/github.com/Amogha-rao/secret-rotator-operator)](https://goreportcard.com/report/github.com/Amogha-rao/secret-rotator-operator)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

## 🎯 What It Does

The Secret Rotator Operator bridges the gap between HashiCorp Vault and Kubernetes by:

- **🔄 Automatic Synchronization**: Continuously monitors Vault secrets and updates corresponding Kubernetes secrets
- **🚀 Rolling Updates**: Automatically triggers pod restarts when secrets change via checksum annotations
- **⏰ Scheduled Polling**: Checks Vault every 10 minutes for secret changes (configurable)
- **🔍 Change Detection**: Only updates Kubernetes secrets when actual changes are detected in Vault
- **🎯 Multi-Workload Support**: Updates Deployments, StatefulSets, DaemonSets, and ReplicaSets
- **🌐 Cross-Namespace**: Can update workloads across different namespaces
- **📊 Status Tracking**: Maintains rotation history, timestamps, and checksums for audit purposes
- **🔐 Secure**: Uses Vault's authentication mechanisms and Kubernetes RBAC
- **⚡ Zero Downtime**: Performs rolling updates without service interruption

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Vault Server  │◄───│ Secret Rotator  │───►│ Kubernetes API  │
│                 │    │   Operator      │    │                 │
│  /secret/data/  │    │                 │    │ Secret Objects  │
│   myapp/db      │    │ Polls every     │    │                 │
└─────────────────┘    │  10 minutes     │    └─────────────────┘
                       └─────────────────┘
                               ▲                        │
                               │                        ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │ SecretRotation  │    │   Workloads     │
                       │ Custom Resource │    │                 │
                       │                 │    │ • Deployments   │
                       │ vaultPath:      │    │ • StatefulSets  │
                       │ targetSecret:   │    │ • DaemonSets    │
                       │ targetWorkloads:│    │ • ReplicaSets   │
                       └─────────────────┘    └─────────────────┘
                                                      ▲
                               Automatic Rolling Updates with
                               Checksum Annotations
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

### 3. Create a SecretRotation Resource with Workload Updates

```yaml
# config/samples/secrets_v1alpha1_secretrotation.yaml
apiVersion: secrets.github.com/v1alpha1
kind: SecretRotation
metadata:
  name: myapp-database-rotation
  namespace: default
spec:
  vaultPath: "secret/data/myapp/database"     # Vault KV v2 path
  targetSecret: "myapp-database-secret"       # Kubernetes secret name
  targetWorkloads:                            # 🆕 Workloads to update when secrets change
    - kind: Deployment
      name: myapp-api
    - kind: StatefulSet  
      name: myapp-cache
    - kind: DaemonSet
      name: log-collector
      namespace: monitoring                   # Cross-namespace support
  annotationPrefix: "myapp.io/"              # 🆕 Custom annotation prefix (optional)
```

```bash
kubectl apply -f config/samples/secrets_v1alpha1_secretrotation.yaml
```

### 4. Verify the Secret and Rolling Updates

```bash
# Check if the secret was created
kubectl get secret myapp-database-secret -o yaml

# Check the SecretRotation status
kubectl get secretrotation myapp-database-rotation -o yaml

# Watch workload annotations get updated
kubectl get deployment myapp-api -o jsonpath='{.spec.template.metadata.annotations}'
```

## 📖 Usage Examples

### Basic Database Credentials with Automatic Rolling Updates

```yaml
apiVersion: secrets.github.com/v1alpha1
kind: SecretRotation
metadata:
  name: postgres-creds
spec:
  vaultPath: "secret/data/production/postgres"
  targetSecret: "postgres-credentials"
  targetWorkloads:
    - kind: Deployment
      name: api-server
    - kind: Deployment
      name: worker-service
```

### Multi-Environment Setup

```yaml
# Production Frontend
---
apiVersion: secrets.github.com/v1alpha1
kind: SecretRotation
metadata:
  name: frontend-api-keys
  namespace: production
spec:
  vaultPath: "secret/data/production/frontend/api-keys"
  targetSecret: "frontend-secrets"
  targetWorkloads:
    - kind: Deployment
      name: frontend-app
    - kind: Deployment
      name: cdn-worker
  annotationPrefix: "frontend.company.io/"

# Backend service with cross-namespace updates
---
apiVersion: secrets.github.com/v1alpha1
kind: SecretRotation
metadata:
  name: backend-service-creds
  namespace: production  
spec:
  vaultPath: "secret/data/production/backend/service-account"
  targetSecret: "backend-credentials"
  targetWorkloads:
    - kind: StatefulSet
      name: backend-api
    - kind: DaemonSet
      name: metrics-collector
      namespace: monitoring
    - kind: Deployment
      name: backup-service
      namespace: utilities
```

### Complete Application Example

```yaml
# Secret rotation configuration
---
apiVersion: secrets.github.com/v1alpha1
kind: SecretRotation
metadata:
  name: database-secrets-rotator
  namespace: production
spec:
  vaultPath: "secret/data/production/database"
  targetSecret: "database-credentials"
  targetWorkloads:
    - kind: Deployment
      name: api-server
    - kind: StatefulSet
      name: database-proxy
  annotationPrefix: "database.company.io/"

# Your deployment that uses the secret
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-server
  namespace: production
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api-server
  template:
    metadata:
      labels:
        app: api-server
      # 🔄 The operator automatically adds/updates this annotation:
      # annotations:
      #   database.company.io/secret-checksum: "abc123def456"
    spec:
      containers:
      - name: api
        image: myapp:latest
        env:
        - name: DB_USERNAME
          valueFrom:
            secretKeyRef:
              name: database-credentials
              key: username
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: database-credentials
              key: password
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
| `targetWorkloads` | []WorkloadReference | List of workloads to update when secrets change | ❌ |
| `annotationPrefix` | string | Custom prefix for checksum annotations | ❌ |

### WorkloadReference Fields

| Field | Type | Description | Required |
|-------|------|-------------|----------|
| `kind` | string | Workload type (Deployment/StatefulSet/DaemonSet/ReplicaSet) | ✅ |
| `name` | string | Name of the workload | ✅ |
| `namespace` | string | Namespace of the workload (defaults to SecretRotation namespace) | ❌ |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `lastRotation` | timestamp | Last time the secret was updated |
| `secretChecksum` | string | SHA256 checksum of current secret data |
| `updatedWorkloads` | []string | List of successfully updated workloads |

## ⚙️ How It Helps

### 🔄 Automated Secret Rotation with Rolling Updates
- **Problem**: Manual secret rotation requires coordinating secret updates AND application restarts
- **Solution**: Automatically detects secret changes AND triggers rolling updates of all dependent workloads

### 🔒 Enhanced Security with Zero Manual Intervention  
- **Problem**: Secrets become stale and applications may cache old credentials
- **Solution**: Ensures applications are automatically restarted with fresh credentials from Vault

### 📈 Operational Efficiency Across Multiple Services
- **Problem**: Large microservice architectures require updating dozens of deployments when secrets rotate
- **Solution**: Single SecretRotation resource can update unlimited workloads across namespaces

### 🚫 True Zero Application Downtime
- **Problem**: Traditional secret rotation causes service disruption
- **Solution**: Uses Kubernetes rolling updates to ensure continuous service availability

### 🎯 GitOps and Infrastructure as Code Friendly
- **Problem**: Secret rotation processes are often manual runbooks
- **Solution**: Declarative configuration that integrates with GitOps workflows

## 🔄 How Rolling Updates Work

When secrets change in Vault:

1. **🔍 Detection**: Operator calculates SHA256 checksum of new secret data
2. **📝 Secret Update**: Kubernetes secret is updated with new values  
3. **🏷️ Annotation Update**: Each target workload gets updated with new checksum annotation:
   ```yaml
   annotations:
     secrets.github.com/secret-checksum: "new-checksum-value"
   ```
4. **🔁 Rolling Update**: Kubernetes sees the pod template change and performs rolling update
5. **♻️ Pod Restart**: New pods start with updated environment variables/mounted secrets
6. **✅ Completion**: Old pods are terminated after new pods are healthy

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

# Check specific status fields
kubectl get secretrotation myapp-database-rotation -o jsonpath='{.status.secretChecksum}'
kubectl get secretrotation myapp-database-rotation -o jsonpath='{.status.updatedWorkloads}'
```

### Verify Workload Updates
```bash
# Check if deployment annotation was updated
kubectl get deployment myapp-api -o jsonpath='{.spec.template.metadata.annotations}'

# Watch rolling update progress
kubectl rollout status deployment/myapp-api
```

### Common Issues

**1. Vault Connection Failed**
```
Error: failed to read from Vault
```
- Verify `VAULT_ADDR` and `VAULT_TOKEN`
- Check network connectivity to Vault

**2. Permission Denied - Secrets**
```
Error: failed to create Kubernetes Secret
```
- Verify RBAC permissions for `secrets` in target namespaces

**3. Permission Denied - Workloads**
```
Error: failed to update workload
```
- Verify RBAC permissions for workload types (`deployments`, `statefulsets`, etc.)
- Check cross-namespace permissions if using different namespaces

**4. Workload Not Found**
```
Error: deployment.apps "myapp" not found
```
- Verify workload names and namespaces in `targetWorkloads`
- Ensure workloads exist before creating SecretRotation

**5. Secret Not Found**
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

- [x] ✅ **Automatic workload rolling updates with checksum annotations**
- [x] ✅ **Multi-workload support (Deployment, StatefulSet, DaemonSet, ReplicaSet)**
- [x] ✅ **Cross-namespace workload updates**
- [ ] Support for different Vault authentication methods (Kubernetes, AWS IAM)
- [ ] Configurable polling intervals via CRD spec
- [ ] Webhook-based immediate rotation triggers
- [ ] Support for selective key synchronization
- [ ] Metrics and Prometheus integration
- [ ] Helm chart for easy installation
- [ ] Support for Jobs and CronJobs
- [ ] Annotation-based workload discovery

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

