# Thumbla Security & Correctness Hardening Pass

**Date:** 2026-07-08
**Status:** Approved (design)
**Source:** Adversarial code review of the "major security enhancements and Echo to Fiber migration" (commits `1fe86e0`, `e95a22c`, `b6930ae`).

## Problem

The "security enhancements and Echo→Fiber migration" introduced controls that are bypassable or non-functional:

- **SSRF** via redirect-following and DNS-rebinding TOCTOU in `fetchers/http.go`, plus an entirely separate, unprotected SSRF vector in `manipulators/paste.go` (raw `http.Get` on a user-controlled URL).
- **Image-decode OOM** — dimension checks run *after* full decode (raster) or after allocating the pixel buffer (SVG); fetchers S3/GS/DS/R2 read bodies with no `LimitReader` and leak object bodies.
- **Redis cache panic + unbounded growth** — `redis.go:Get` JSON-unmarshals `[]image.Rectangle` into `interface{}`, so `facecrop.go`'s unchecked `.( []image.Rectangle )` panics on every Redis-backed hit; TTL is `0`; LRU trim `ZPopMin`s without `DEL`ing data keys.
- **Silent wrong output** — output quality/lossless/exact/encoder params never reach the response (Fiber `c.Set` Locals vs `c.Get` request-header mismatch), and `validateManipulatorParameter` treats the `r` resampling filter as numeric RGB, silently dropping the entire manipulator chain on `r=lanczos`.
- **Medium gaps** — Redis password YAML tag typo, `restrictPaths` `..` prefix bypass, CacheControl broken for non-trailing-slash paths, middleware ordering (logger before recover), rate-limiter trusts spoofable `c.IP()`.
- **Tests** do not assert the claimed security properties (SSRF tests all set `disableSSRFProtection: true`); manipulator tests bypass `parseManipulators`.

## Scope

Full hardening pass (option C from brainstorming): Critical + High + all medium items. **No new features.** Out of scope: wiring cache into the main request path beyond facecrop, new storage backends, new manipulators, and anything not in the review.

## Architecture & Components

### 1. Shared SSRF-safe HTTP client — new `httpclient/` package

A single `*http.Client` used by both `fetchers/http.go` and `manipulators/paste.go`. Relocates the existing `isPrivateOrLocalIP`/`isDangerousPort` helpers here.

- **Custom `http.Transport.DialContext`:** resolves the address itself and refuses loopback/private/link-local IPs and dangerous ports at *connect* time. This closes the redirect bypass (re-validated per connection) and the DNS-rebinding TOCTOU (a single resolution — the one that matters).
- **`CheckRedirect`:** re-runs host/port + `restrictPaths` validation on each `Location`. `restrictPaths` uses `path.Clean` and verifies the cleaned path stays under an allowed prefix (fixes the `http://host/allowed/../secret` bypass).
- **`restrictHosts` / `restrictPaths` / size limit** wired into the client so every caller inherits them.
- **Uniform `io.LimitReader(reader, maxImageSizeBytes+1)`** for any HTTP body read; reject on overflow.
- **Test-only bypass:** honor the existing `disableSSRFProtection` flag so `http_test.go` keeps working; new tests added that do **not** disable it.

### 2. Image-decode memory safety — `handlers/imagehandler.go` + fetchers

- **Raster formats:** call `*DecodeConfig` first; reject if `> maxImageDimension` or `≤ 0`; then `*Decode`. This bounds the decompressed pixel buffer.
- **SVG:** clamp `ViewBox.W/H` against `maxImageDimension` **before** `NewRGBA64`; reject non-positive; check `err` immediately after `ReadIconStream` (fixes the nil-deref).
- **All fetchers (S3/GS/DS/R2):** `defer output.Body.Close()`; wrap reads in `io.LimitReader(reader, maxImageSizeBytes+1)`; reject overflow. The handler closes `imageBody` for streaming fetchers (DS/R2 return a raw `io.ReadCloser`).

### 3. Cache layer — `cache/` + `config/` + `facecrop.go`

- **Typed cache:** add `GetBytes`/`SetBytes` to the `Cache` interface (bytes in, bytes out) so `facecrop.go` marshals/unmarshals a concrete `[]image.Rectangle` (via `encoding/gob`); use a two-value assertion `faces, ok := ...; if !ok { re-detect }`. Fixes the panic and the `Contains`→`Get` TOCTOU.
- **Real TTL:** new `Cache.Redis.TTL` config field with a sane default; `Set` uses it (not `0`).
- **LRU trim:** `DEL` data keys alongside `ZPopMin` (fixes unbounded Redis growth).
- **Redis password:** fix YAML tag to `yaml:"password"` (fixes silent auth loss).

### 4. Output + manipulator validation — `manipulators/` + `handlers/imagehandler.go`

- **Output params:** write/read quality/lossless/exact/encoder via `c.Locals(...)` consistently on both sides; stop trusting request headers. Encoder allowlist (`jpeg|png|webp|gif`); reject unknown (no empty 200 body). Fix the `image/web` typo (`output.go`).
- **Per-manipulator `Validate(params)`** method on the `Manipulator` interface; remove the global numeric `numericParams` name-map. `resize`/`fit` accept `r` as a string filter name and validate `w`/`h` as numeric. Fixes the silent chain-drop on `r=lanczos`.

### 5. Medium items — `thumbla.go`, `config/`, `handlers/imagehandler.go`

- **Middleware order:** register `recover.New()` **before** `middleware.New()`.
- **CacheControl:** pass the already-computed base path (`rawRequestPath[0:idx("/:url")]`) to `GetPathConfigByPath`, not `c.Route().Path`.
- **Rate-limit identity:** add a `TrustedProxies` config block + Fiber `TrustedProxyConfig` / `ProxyHeader`. Default = trust nothing (peer IP, unspoofable); operator opts in per deployment. Document in `config-example.yml`.

## Data Flow

```
Request
  → recover (registered first)
  → logger
  → routing
  → HandleImage
      → parseManipulators (per-manipulator Validate)
      → fetcher (shared SSRF client + LimitReader + body close)
      → loadImage (DecodeConfig → decode; SVG clamp before alloc)
      → manipulators (paste uses shared SSRF client)
      → output (Locals, not request headers)
  → response (CacheControl from base path)
Rate limit keyed on trusted c.IP().
Facecrop cache: typed round-trip with TTL.
```

## Error Handling

| Condition | Response |
|---|---|
| Malformed file params / SVG / `_blank` (no `|`) | 400 (length-check `params`) |
| Oversized dimensions (DecodeConfig or SVG ViewBox) | 400 before allocation |
| SSRF redirect / DNS-rebind target | blocked at dial → 502/400 |
| Unknown encoder / `X-Encoder: guetzli` | 400, not empty 200 |
| Fetcher body overflow (`LimitReader` exceeded) | 413 (payload too large) |
| Redis auth required but unconfigured | surface error (no silent swallow) |

## Testing

- **SSRF** (without disabling protection): redirect-to-`127.0.0.1` rejected; `isPrivateOrLocalIP` against `169.254.x`, `127.0.0.1`, `10.x`; `restrictPaths` `..` rejected.
- **Decode memory:** oversized PNG/SVG rejected before allocation (assert via an allocation-counting or size-cap test); fetcher `LimitReader` overflow rejected; bodies closed (leak-detecting fetcher test with `conntrack`/open-handle count).
- **Cache:** Redis-backed facecrop round-trip returns `[]image.Rectangle` without panic; TTL applied to `SET`; trim `DEL`s data keys.
- **Output:** `output:f=jpg,q=45` produces JPEG with quality 45 (decode + inspect); unknown encoder → 400; client-sent `X-Quality` request header is ignored.
- **Manipulator:** `resize:w=100,r=lanczos/output:f=jpg` end-to-end returns the resized image (through `parseManipulators`, not bypassed).
- **Medium:** middleware ordering; CacheControl for a non-trailing-slash path; rate-limit ignores a spoofed `X-Forwarded-For` when untrusted.

## Out of Scope

- Wiring cache into the main request path beyond facecrop (separate feature).
- New storage backends, new manipulators.
- Any change not backed by a review finding.
