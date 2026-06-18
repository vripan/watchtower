# Proposal: read-only "check for updates" HTTP API endpoint (`GET /v1/check`)

## Problem

Watchtower's HTTP API today offers:

- `POST /v1/update` — triggers an update (side-effecting: pulls + recreates containers).
- `GET /v1/metrics` — Prometheus counters about scan activity.
- `GET /v1/containers` — read-only, lists each watched container's **current** running
  image digest, but does **not** query the registry.

None of these lets an external application answer the simple question **"is a newer
image available?"** without either triggering an actual update or re-implementing the
registry-digest comparison client-side (which means duplicating Watchtower's registry
credentials and mirror/auth handling).

A common use case: an app or dashboard wants to show an "update available" banner using
Watchtower's **existing** registry credentials — no duplicated creds, no side effects.

## Proposed solution: `GET /v1/check`

A read-only endpoint, behind the existing `WATCHTOWER_HTTP_API_TOKEN`, that scans the
watched containers and compares each running image's digest against the registry, with
**no pull and no recreate**. It reuses the exact staleness mechanism Watchtower already
runs internally (`pkg/registry/digest.CompareDigest`, the HEAD-request digest comparison
that `imageClient.shouldSkipPull` uses to avoid unnecessary pulls).

Enabled with `--http-api-check` / `WATCHTOWER_HTTP_API_CHECK`, mirroring the existing
`--http-api-containers` flag.

### Example response

```json
{
  "checked": "2026-06-17T11:30:45Z",
  "containers": [
    {
      "name": "nginx",
      "image": "nginx:latest",
      "current_digest": "sha256:1111...",
      "latest_digest": "sha256:2222...",
      "update_available": true
    }
  ],
  "count": 1,
  "api_version": "v1"
}
```

Per-container registry errors are reported with an `"error"` field on that entry
(and `update_available: false`) rather than failing the whole response, so one
unreachable registry doesn't blank out the rest.

### Why a new endpoint rather than overloading `/v1/update`

`/v1/check` is a `GET`, idempotent, and safe to call concurrently. It is the natural
complement to the existing read-only `/v1/containers` endpoint — `/v1/containers` reports
what is running; `/v1/check` reports whether something newer exists.

## Alternative considered: `dry_run=true` on `/v1/update`

A `POST /v1/update?dry_run=true` parameter that performs the staleness scan and returns
the same structured result without pulling/recreating.

Trade-offs:

- **Against:** `/v1/update` is a side-effecting `POST` guarded by a single-update
  concurrency lock (it returns `429` when an update is already running). A read-only
  query would either contend for / block on that lock or need a special bypass, muddying
  the endpoint's semantics. `GET` is also the more correct verb for a pure read.
- **For:** keeps the surface area to a single endpoint and reuses the `image=` targeting
  query parameter already supported by `/v1/update`.

I lean toward the dedicated `GET /v1/check` for clarity and consistency with
`/v1/containers`, but I'm happy to implement whichever you prefer before opening the PR.

## Scope notes / open questions

- v1 compares against the canonical registry host (same as a plain `CompareDigest` call).
  Honoring Docker daemon registry **mirrors** (as the pull path does via
  `resolveRegistryMirrorConfig`) could be a follow-up.
- Honors the same container filter Watchtower is already running with.
- Locally-built images (empty `RepoDigests`) report `update_available: false` with an
  empty `latest_digest`, matching how `CompareDigest` already treats them.
