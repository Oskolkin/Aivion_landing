import json
import subprocess
from pathlib import Path

sql = (
    "SELECT payload_json::text FROM ai_pipeline_artifacts "
    "WHERE pipeline_run_id=193 AND kind='unified_analysis_output_raw' "
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
    text=True,
)
obj = json.loads(raw.strip())
if isinstance(obj, dict) and "dashboard" not in obj:
    for k in ("output", "payload", "body", "text", "content"):
        v = obj.get(k)
        if isinstance(v, str):
            try:
                obj = json.loads(v)
                break
            except json.JSONDecodeError:
                pass
        if isinstance(v, dict):
            obj = v
            break

narr = ((obj.get("dashboard") or {}).get("ad_efficiency") or {}).get("narrative") or {}
out = {
    "headline": narr.get("headline"),
    "summary": narr.get("summary"),
    "metric_claims": narr.get("metric_claims"),
    "action_count": len(obj.get("actions") or []),
    "schema_version": obj.get("schema_version"),
    "analysis_outcome": obj.get("analysis_outcome"),
}
path = Path("docs/audits/real-run-2026-07-22-v125/run-193-ad-narrative.json")
path.write_text(json.dumps(out, ensure_ascii=False, indent=2), encoding="utf-8")
print(path)
print(json.dumps(out, ensure_ascii=False, indent=2)[:2500])
