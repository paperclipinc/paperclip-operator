---
title: Runtime-configured features
summary: Paperclip features that are configured in the app UI/API at runtime, not via the operator CRD
---

# Runtime-configured features

Some Paperclip capabilities are configured inside the running application (UI,
API, or company secrets) rather than through the operator's `Instance` CRD. The
operator deliberately does not model these, because the app does not read them
from environment variables. This page documents how to use them alongside an
operator-managed instance.

## MCP server

The standalone MCP server ships as the `@paperclipai/mcp-server` package. It is
a **stdio binary**, not a networked service: it has no listening port and the
operator cannot expose it as a Kubernetes `Service`. An MCP client launches it
as a subprocess and talks to it over stdin/stdout. It connects to a Paperclip
instance via two environment variables you set on the client:

| Variable | Description |
|----------|-------------|
| `PAPERCLIP_API_URL` | Base API URL of the instance (must end in `/api`). For an in-cluster client, the operator-created Service: `http://<instance-name>.<namespace>.svc.cluster.local:3100/api`. |
| `PAPERCLIP_API_KEY` | A Paperclip API key (created in the app). |

Optional: `PAPERCLIP_COMPANY_ID`, `PAPERCLIP_AGENT_ID`, `PAPERCLIP_RUN_ID`.

Example MCP client config entry:

```json
{
  "mcpServers": {
    "paperclip": {
      "command": "paperclip-mcp-server",
      "env": {
        "PAPERCLIP_API_URL": "https://paperclip.example.com/api",
        "PAPERCLIP_API_KEY": "pcp_..."
      }
    }
  }
}
```

## Sandbox providers (Modal, Cloudflare)

Sandbox execution providers are selected per-Environment as records in the app
(UI/API), and most providers read their credentials from plugin config / company
secrets, **not** from pod environment variables. The operator can only wire the
one provider that has an environment fallback:

- **E2B** - `spec.adapters.e2b.apiKeySecretRef` sets `E2B_API_KEY`. You still
  enable the `@paperclipai/plugin-e2b` plugin and select E2B for the Environment
  in the UI.
- **Modal** - configured in plugin config (`tokenId`, `tokenSecret`); plugin
  workers do not inherit pod env, so `MODAL_TOKEN_ID`/`MODAL_TOKEN_SECRET` on the
  pod are not honored. Configure these in the Paperclip UI.
- **Cloudflare** - configured with `bridgeBaseUrl` + `bridgeAuthToken` in plugin
  config; Cloudflare account credentials live in the self-hosted bridge worker,
  not in Paperclip. Configure in the UI.

For in-cluster sandboxing without an external provider, use the operator's
Kubernetes-native sandbox instead: `spec.adapters.cloudSandbox`.

## Environments (SSH / local / sandbox)

The Environments feature (local, SSH-backed remote, and sandboxed execution
targets) is stored as runtime DB records created through the app UI/API. There
is no environment-variable surface, so the operator does not configure them. SSH
remote targets are defined in the UI with: `host`, `port` (default 22),
`username`, `remoteWorkspacePath`, and a `privateKeySecretRef` referencing a
company secret.

## First admin

If you set `spec.auth.adminUser`, the operator runs a bootstrap Job that creates
that admin account directly - this is the credentialed path for unattended
provisioning.

If you omit `spec.auth.adminUser`, the app's native first-admin (board-claim)
flow takes over: in `authenticated` mode, the first human to authenticate can
claim instance ownership. No operator configuration is required for that path.
