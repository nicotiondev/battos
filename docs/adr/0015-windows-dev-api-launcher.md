# ADR-0015: Launcher dev del API en Windows

## Estado

Aceptado.

## Contexto

Durante Fase 3B, el binario `battos.exe` firmado con el certificado local de
desarrollo puede ejecutarse en Windows. Sin embargo, `battos-api.exe` firmado
con el mismo certificado puede ser bloqueado por Windows Application Control en
algunos intentos de ejecucion.

El certificado local actual se instala en `TrustedPublisher`, pero no en
`TrustedRoot` por defecto. Agregarlo a `TrustedRoot` reduce friccion local, pero
tambien aumenta la superficie de confianza del equipo. Para desarrollo no es
razonable exigir ese paso como default.

## Decision

En desarrollo local Windows, el API se levanta oficialmente con:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-battos-api-dev.ps1 -StopExisting -Background -Wait
```

Ese script ejecuta `go run ./apps/api/cmd/api`, configura `DATABASE_URL` de
desarrollo si no existe, usa `BATTOS_API_PORT`, espera `/status` y evita el
desfase entre CLI nueva y API vieja.

Para probar estados degradados sin Postgres:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-battos-api-dev.ps1 -Port 8001 -StopExisting -Background -Wait -NoDatabase
```

`battos-api.exe` queda fuera del flujo dev diario hasta tener firma/release
formal o instalador que resuelva la confianza de forma explicita.

## Consecuencias

- El flujo dev es reproducible sin confiar el certificado como root.
- El usuario no necesita bajar la seguridad del equipo para seguir avanzando.
- El smoke test dev valida el sistema contra el API realmente levantado.
- El release v0.1 debe volver a resolver distribucion/firma de `battos-api.exe`
  o empaquetado equivalente.

## Verificacion

Comandos verificados:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\start-battos-api-dev.ps1 -StopExisting -Background -Wait
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-battos-dev.ps1 -RequireDatabase
```

Tambien se verifico el modo degradado sin DB con `-NoDatabase`, donde Work
Board responde `503` accionable en vez de `404`.
