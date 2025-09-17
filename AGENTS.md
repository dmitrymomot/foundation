# Repository Guidelines

## Project Structure & Docs

`core/` packages power the HTTP framework, `middleware/` wraps cross-cutting policies, `pkg/` offers focused utilities (JWT, TOTP, async, observability), and `integration/` contains adapters for data stores, email, and object storage. Tests sit beside sources as `_test.go`, while each package ships a `doc.go` overview—run `go doc github.com/dmitrymomot/foundation/<package>` or `go doc -all` for deep dives.

## Build, Test, and Development Commands

- `go test -p 1 -count=1 -race -cover ./...` mirrors CI expectations before a push.
- `go test ./pkg/totp -run TestGenerate` (pattern) keeps focused iteration tight.
- `go fmt ./...` and `goimports -w -local github.com/dmitrymomot/foundation .` align formatting and imports.
- `go vet ./...` and `golangci-lint run` enforce the static analysis gate.

## Coding Style & Core Patterns

Use tabs, keep package names lowercase, and export concise PascalCase APIs. Preserve the generics-first design for handlers and contexts, configure components with functional options (`WithStore`, `WithTransport`, etc.), and rely on interfaces for pluggable transports or stores. Run formatters before committing so reviews center on behavior.

## Testing Guidelines

Tests stay black-box: use `package_test`, target exported constructors or handlers, and avoid reaching into unexported helpers. Table suites are restricted—reserve them for wide input grids like `pkg/clientip/clientip_test.go`; otherwise write explicit subtests with Arrange/Act/Assert and guard `t.Parallel()` behind isolated state. Prefer stdlib assertions, importing `testify/require` only when mocks demand it. Watch coverage with `go test -cover ./...` and protect the hot paths in `core/`.

## Commit & Pull Request Guidelines

Stick to the Conventional Commit history (`feat(session): …`, `refactor(vectorizer): …`). Keep each commit narrowly scoped and include generated or formatted artifacts. PRs need context, solution notes, verification (`go test -p 1 -count=1 -race -cover ./...`), linked issues, and evidence—screens, logs, or API samples—when behavior changes.

## Architecture & Stack Notes

Foundation targets secure, multi-tenant B2B SaaS. templ + Tailwind CSS + HTMX power rendering, PostgreSQL with `sqlc` backs persistence, and active development means carefully-scoped breaking changes are acceptable. Security-first defaults—validation, sanitization, rate limiting—must remain intact.

## Security & Configuration Tips

Never commit secrets; rely on `core/config` and environment variables. Mirror adapter samples such as `integration/database/pg` when wiring services, keep provider keys in a local `.env`, and audit new dependencies for licensing and CVE risk. Document any hardening steps in your PR so operators can reproduce them.
