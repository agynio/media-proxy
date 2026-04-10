# Media Proxy

Media Proxy is a Go HTTP service that proxies external media and platform files for secure inline display.

## Endpoints

- `GET /proxy?url={url}&size={size}`
- `GET /healthz`

## Configuration

| Variable | Required | Description | Default |
| --- | --- | --- | --- |
| `LISTEN_ADDR` | no | Address to bind the HTTP server | `:8080` |
| `OIDC_ISSUER_URL` | yes | OIDC issuer URL |  |
| `OIDC_CLIENT_ID` | yes | OIDC client ID |  |
| `USERS_GRPC_TARGET` | yes | Users service gRPC target |  |
| `FILES_GRPC_TARGET` | yes | Files service gRPC target |  |
| `CORS_ALLOWED_ORIGIN` | no | Allowed CORS origin | `https://agyn.dev` |
| `MAX_RESPONSE_SIZE` | no | Max proxied response size (bytes) | `52428800` |
| `REQUEST_TIMEOUT` | no | Timeout for origin requests | `30s` |
| `MAX_REDIRECTS` | no | Max redirect hops for external URLs | `3` |
| `MAX_IMAGE_SIZE` | no | Max allowed `size` parameter value | `4096` |
