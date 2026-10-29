import json
import subprocess
from pathlib import Path


def fetch(kind: str):
    sql = (
        "SELECT payload_json::text FROM ai_pipeline_artifacts "
        f"WHERE pipeline_run_id=195 AND kind='{kind}' "
        "ORDER BY id DESC LIMIT 1;"
    )
    raw = subprocess.check_output(
        [
            "docker",
            "exec",
            "marketplace_ai_postgres",
            "psql",
            "-U",
            "postgres",
            "-d",
            "marketplace_ai",
            "-t",
            "-A",
            "-c",
            sql,
        ],
        encoding="utf-8",
        errors="replace",
    ).strip()
    return json.loads(raw) if raw else None


out_dir = Path("docs/audits/real-run-2026-07-22-dee1c61")
raw = fetch("unified_analysis_output_raw")
val = fetch("unified_analysis_output_validation")
req = fetch("unified_provider_request_summary")
narr = (((raw or {}).get("dashboard") or {}).get("ad_efficiency") or {}).get("narrative") or {}
summary = {
    "schema_version": (raw or {}).get("schema_version"),
    "headline": narr.get("headline"),
    "summary": narr.get("summary"),
    "metric_claims": [
        {"metric_code": c.get("metric_code"), "value": c.get("value")}
        for c in (narr.get("metric_claims") or [])
    ],
    "threshold_claims": narr.get("threshold_claims"),
    "action_count": len((raw or {}).get("actions") or []),
}
(out_dir / "run-195-ad-narrative.json").write_text(
    json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8"
)
(out_dir / "output-validation.json").write_text(
    json.dumps(val, ensure_ascii=False, indent=2), encoding="utf-8"
)
(out_dir / "provider-request-summary.json").write_text(
    json.dumps(req, ensure_ascii=False, indent=2), encoding="utf-8"
)
print(json.dumps(summary, ensure_ascii=False, indent=2)[:5000])
print("---VALIDATION---")
print(json.dumps(val, ensure_ascii=False, indent=2)[:5000])
