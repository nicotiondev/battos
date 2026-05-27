# BattOS OpenAPI

`openapi.yaml` is the contract source of truth introduced in Phase 3A.

Current implementation coverage:

- `GET /health`, `GET /version`, `GET /status`
- `/memory/*`

ADR-0013 bearer middleware is implemented for protected current endpoints.
Local development explicitly uses disabled auth on loopback only; deployments
use `auth.mode: token` and `BATTOS_API_TOKEN`.

Operations marked with `x-battos-phase` define the v0.1 API that later
phases implement. A declared endpoint is not available until its phase is
completed and verified.

Generation workflow will be wired before server/client code is generated:

```powershell
./scripts/generate.ps1
```

Generated Go and TypeScript clients must never be edited manually.

Contract parsing and boundary checks run through:

```powershell
go test ./apps/api/internal/contract
```
