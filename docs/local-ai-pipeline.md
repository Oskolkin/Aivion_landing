# Local AI pipeline (Windows PowerShell)

This guide describes the supported way to run the unified AI pipeline on a local Windows machine without manually setting `$env:*` in every new PowerShell session.

## First-time setup

1. Copy the environment template:

```powershell
Copy-Item .env.example .env.local
```

2. Edit `.env.local` once and fill secrets / non-default values:

- `OPENAI_API_KEY` — required for real AI runs
- `AI_PROXY_URL` — optional HTTP proxy for OpenAI (for example `http://127.0.0.1:10809`)
- `S3_SECRET_KEY` — only if you do not use the default local MinIO password from `docker-compose.yml`

Infrastructure defaults (`DATABASE_URL`, `REDIS_ADDR`, MinIO endpoint) match `docker-compose.yml` and are applied automatically when missing.

3. Start local infrastructure:

```powershell
docker compose up -d postgres redis minio
```

4. Validate environment:

```powershell
.\scripts\dev\check-env.ps1
```

## Configuration precedence

From highest to lowest priority:

1. Explicit PowerShell launcher parameters
2. Variables already set in the current process environment
3. `.env.local`
4. `.env.development.local` (if present)
5. `.env`
6. Docker Compose discovery (`postgres`, `redis`, `minio` services)
7. Safe local defaults (non-secret dev values)

Use `-OverrideEnvironment` on launcher scripts to let `.env.local` replace existing process variables.

`.env.local` is gitignored. Never commit real API keys or passwords.

## Full pipeline run

```powershell
.\scripts\dev\run-ai-pipeline.ps1 `
  -SellerID 1 `
  -MetricDate "2026-07-08" `
  -Force
```

Defaults (unless explicitly disabled): `-Dashboard`, `-Actions`, `-AIEnabled`, `-DebugArtifacts`.

Shorter root wrapper:

```powershell
.\run-ai-pipeline.ps1 -SellerID 1 -MetricDate "2026-07-08" -Force
```

The launcher automatically:

- finds the repository root
- loads `.env.local`
- discovers Docker endpoints when needed
- runs `check-ai-run-readiness`
- blocks when another `running` pipeline exists for the same seller/date
- runs `go run ./cmd/devtools run-ai-pipeline`
- prints `pipeline_run_id` and export command

## Resume / stage restart

Restart Actions from an existing run:

```powershell
.\scripts\dev\run-ai-pipeline.ps1 `
  -ResumeRunID 74 `
  -RestartFromStage actions
```

Supported `-RestartFromStage` values match the Go devtool:

- `actions`
- `global_context`
- `shared_chunk_validation`
- `shared_chunk_analysis`

Resume cannot be combined with `-Force`.

## Inspector export

```powershell
.\scripts\dev\export-ai-run.ps1 -RunID 78 -DiagnosticSummary
```

Open in browser:

```powershell
.\scripts\dev\export-ai-run.ps1 -RunID 78 -Open
```

Or auto-export after a successful run:

```powershell
.\scripts\dev\run-ai-pipeline.ps1 -SellerID 1 -MetricDate "2026-07-08" -Force -ExportInspector
```

## Stale `running` run

If a previous run was interrupted and the database still shows `running`:

```powershell
.\scripts\dev\run-ai-pipeline.ps1 `
  -SellerID 1 `
  -MetricDate "2026-07-08" `
  -MarkStaleRunFailed `
  -Confirm
```

This only runs when no active `run-ai-pipeline` process is detected.

## New PowerShell window

Open a new terminal and run the launcher directly. Manual `$env:APP_ENV`, `$env:DATABASE_URL`, `$env:REDIS_ADDR`, and OpenAI settings are not required when `.env.local` exists.

## Production note

Production API startup does not depend on `.env.local`. Local dotenv loading is limited to dev scripts and non-production `LoadEnvFiles()` behavior in development binaries.
