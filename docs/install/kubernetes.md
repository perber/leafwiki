# Deploy LeafWiki on Kubernetes

This example shows how to run LeafWiki on Kubernetes: a `Deployment` backed by a `PersistentVolumeClaim` for `/app/data`, a `Service`, and an `Ingress` to expose it. It was contributed by [@jforman](https://github.com/jforman) in [#1595](https://github.com/perber/leafwiki/issues/1595) and verified end-to-end in a local [kind](https://kind.sigs.k8s.io/) cluster before being added here.

---

## 1. Secret for admin password and JWT secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: leafwiki-secret
type: Opaque
stringData:
  admin_password: "changeme"
  jwt_secret: "generate-a-long-random-secret"
```

## 2. Persistent storage

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: leafwiki-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
  storageClassName: CHANGEME # your cluster's StorageClass
```

## 3. Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: leafwiki
  labels:
    app: leafwiki
spec:
  replicas: 1
  selector:
    matchLabels:
      app: leafwiki
  template:
    metadata:
      labels:
        app: leafwiki
    spec:
      # Required — see "Gotcha: enableServiceLinks" below.
      enableServiceLinks: false
      containers:
        - name: leafwiki
          image: ghcr.io/perber/leafwiki:latest
          args:
            - "--host=0.0.0.0"
            # Only needed if nothing in front of LeafWiki terminates TLS and
            # forwards `X-Forwarded-Proto: https` — see the note below.
            - "--allow-insecure=true"
          ports:
            - containerPort: 8080
          env:
            - name: LEAFWIKI_ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: leafwiki-secret
                  key: admin_password
            - name: LEAFWIKI_JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: leafwiki-secret
                  key: jwt_secret
          volumeMounts:
            - name: data
              mountPath: /app/data
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: leafwiki-pvc
```

`replicas` must stay at `1`: LeafWiki's storage model (markdown on disk plus a set of SQLite indexes) is not designed for multiple instances writing to the same data directory concurrently.

## 4. Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: leafwiki
spec:
  type: ClusterIP
  selector:
    app: leafwiki
  ports:
    - name: http
      port: 8080
      targetPort: 8080
```

## 5. Exposing it

### Option A — Ingress

Works with any ingress controller (tested against [ingress-nginx](https://kubernetes.github.io/ingress-nginx/)):

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: leafwiki
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "50m" # match your upload size needs
spec:
  ingressClassName: nginx
  rules:
    - host: wiki.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: leafwiki
                port:
                  number: 8080
```

Add TLS via your ingress controller / cert-manager as usual (e.g. a `cert-manager.io/cluster-issuer` annotation plus a `tls:` block). Once TLS terminates in front of LeafWiki and `X-Forwarded-Proto: https` is forwarded, drop `--allow-insecure=true` from the Deployment.

### Option B — Gateway API

If your cluster already runs a [Gateway API](https://gateway-api.sigs.k8s.io/) controller (e.g. Traefik in Gateway mode), the equivalent looks like this — as originally proposed in #1595:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: gateway-leafwiki
  annotations:
    cert-manager.io/cluster-issuer: CHANGEME
spec:
  gatewayClassName: traefik # your Gateway API controller's class
  listeners:
    - name: websecure
      hostname: wiki.example.com
      port: 443
      protocol: HTTPS
      tls:
        mode: Terminate
        certificateRefs:
          - name: leafwiki-tls
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: leafwiki
spec:
  parentRefs:
    - name: gateway-leafwiki
      sectionName: websecure
  hostnames:
    - wiki.example.com
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: leafwiki
          port: 8080
```

This wasn't re-verified with a live Gateway API controller (only the Ingress path above was), but the flags and env vars below apply the same way. If your Gateway API implementation terminates TLS and forwards `X-Forwarded-Proto: https` (Traefik does), `--allow-insecure=true` is not needed.

---

## Gotcha: `enableServiceLinks`

Kubernetes injects one environment variable per Service in the same namespace, named `<SERVICE_NAME>_PORT` (e.g. `LEAFWIKI_PORT=tcp://10.96.x.x:8080`) into every pod — this is the legacy "service links" mechanism, still on by default. If your Service is named `leafwiki` (as above), that variable collides with LeafWiki's own `LEAFWIKI_*` environment variable namespace and overrides `--port`, and the server fails to start with `listen tcp: address 0.0.0.0:tcp://10.96.x.x:8080: too many colons in address`.

`enableServiceLinks: false` on the pod spec disables the injection and avoids this entirely. It's not specific to any one Service name — any Service name that happens to prefix-match `LEAFWIKI_*` triggers it, so set it regardless of what you end up naming the Service.

## Non-root containers

To run as a non-root user (see the [Docker non-root example](../../README.md#docker)), add a pod-level `securityContext` with a matching `fsGroup` so the mounted volume is group-writable for that user:

```yaml
spec:
  securityContext:
    runAsUser: 1000
    runAsGroup: 1000
    fsGroup: 1000
```

Whether this is actually required depends on your cluster's `StorageClass`/CSI driver — some (like kind's default) already hand out world-writable volumes, others (most cloud block storage) mount them owned by `root:root`. Setting `fsGroup` covers both cases.

## Git backup / snapshots / SMTP

All other LeafWiki flags and environment variables work the same as anywhere else — see the [README](../../README.md) for the full list. For example, to add git backup with an SSH key:

```yaml
          args:
            - "--host=0.0.0.0"
            - "--git-backup"
            - "--git-backup-remote=git@github.com:username/leafwiki-backup.git"
          env:
            - name: LEAFWIKI_GIT_BACKUP_SSH_KEY
              valueFrom:
                secretKeyRef:
                  name: leafwiki-git-ssh-key
                  key: ssh-privatekey
```

Prefer the `LEAFWIKI_GIT_BACKUP_SSH_KEY` env var (via `secretKeyRef`) over the `--git-backup-ssh-key` flag — flag values are visible in `kubectl describe pod` and process listings.
