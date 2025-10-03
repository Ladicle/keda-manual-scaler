# KEDA Manual External Scaler

Manually control KEDA scaling decisions via a simple HTTP API.

This component implements the KEDA External **Push** Scaler (gRPC) interface and exposes an HTTP endpoint you can call to flip `IsActive` and set a custom metric value in real time. Useful for:

* Demoing how KEDA reacts to push scaler events
* Chaos / failure drills (simulate spikes / drops)
* Reproducing edge conditions in autoscaling logic
* Local or integration test environments without needing a real event source

> Not recommended for production autoscaling logic – it's intentionally "manual" and trusts all callers (no auth layer built in yet).

## Architecture

```
┌───────────┐    HTTP   ┌──────────────────────┐
│ User / CI │  -------> │  Manual Scaler HTTP  │
└───────────┘           │  (updates in-memory) │
HTTP (manual events)    └─────────┬────────────┘
/?active=true&value=25            │ internal channel
                        ┌─────────▼───────────┐  gRPC  ┌────────────┐
                        │  External Push      │<──────>│    KEDA    │
                        │  Scaler (gRPC srv)  │        │  Operator  │
                        └─────────┬───────────┘        └────────────┘
                                  │
                          Scale Target (HPA -> Deployment, etc.)
```

## Getting Started

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: manual-scaler
  labels:
    app: manual-scaler
spec:
  replicas: 1
  selector:
    matchLabels:
      app: manual-scaler
  template:
    metadata:
      labels:
        app: manual-scaler
    spec:
      containers:
        - name: scaler
          image: ghcr.io/Ladicle/keda-manual-scaler:latest
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 8080
            - name: grpc
              containerPort: 8081
---
apiVersion: v1
kind: Service
metadata:
  name: manual-scaler
  labels:
    app: manual-scaler
spec:
  selector:
    app: manual-scaler
  ports:
    - name: http
      port: 8080
      targetPort: http
    - name: grpc
      port: 8081
      targetPort: grpc
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-worker
  labels:
    app: demo-worker
spec:
  replicas: 0
  selector:
    matchLabels:
      app: demo-worker
  template:
    metadata:
      labels:
        app: demo-worker
    spec:
      containers:
        - name: worker
          image: registry.k8s.io/pause:3.9
---
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: demo-worker
spec:
  scaleTargetRef:
    kind: Deployment
    name: demo-worker
  cooldownPeriod: 30
  minReplicaCount: 0
  maxReplicaCount: 5
  triggers:
    - type: external-push
      metadata:
        scalerAddress: manual-scaler.default.svc.cluster.local:8081
        metricName: manual
```

Apply:
```bash
kubectl apply -f manual-scaler-example.yaml
```

Port-forward HTTP (in another terminal) and activate scaling:
```bash
kubectl port-forward deploy/manual-scaler 8080:8080 &
curl 'http://localhost:8080/?name=demo-worker&active=true&value=10'
```

Watch replicas grow:
```bash
kubectl get deploy demo-worker -w
```

Deactivate:
```bash
curl 'http://localhost:8080/?name=demo-worker&active=false&value=0'
```

## Configuration

Config file (YAML) mounted at runtime using `--config /path/config.yaml`:
```yaml
grpcScalerPort: 8081       # gRPC server port (KEDA connects here)
httpAPIPort: 8080          # HTTP API port for manual events
default:
  metricName: manual       # Metric name KEDA will see
  active: false            # Initial IsActive
  targetSize: 1            # Target size (KEDA uses for scaling metric spec)
  metricValue: 0           # Initial metric value
metrics: {}                # (optional) reserved for future static object metrics
```
Helm chart automatically renders this ConfigMap from values.

## HTTP API

Endpoint: `GET /`

Query parameters:
* `active`  (bool, required) – `true` / `false`
* `value`   (int64, required) – metric numeric value
* `name`    (string, optional) – ScaledObject name (matches `metadata.name` in `ScaledObject`). If omitted, updates the global default state.

Examples:
```bash
# Activate (IsActive=true) globally and set metric value=10
curl "http://scaler.example.local/?active=true&value=10"

# Deactivate (IsActive=false) globally and set metric back to 0
curl "http://scaler.example.local/?active=false&value=0"

# Activate only the ScaledObject named 'my-worker' with metric value=42
curl "http://scaler.example.local/?name=my-worker&active=true&value=42"
```
