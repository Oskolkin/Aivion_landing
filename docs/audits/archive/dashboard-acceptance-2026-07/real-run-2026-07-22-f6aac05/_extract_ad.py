import json
import subprocess
from pathlib import Path

sql = (
    "SELECT payload_json::text FROM ai_pipeline_artifacts "
    "WHERE pipeline_run_id=194 AND kind='unified_analysis_output_raw' "
    "ORDER BY id DESC LIMIT 1;"
)
raw = subprocess.check_output(
    [
        "docker", "exec", "marketplace_ai_postgres", "psql",
        "-U", "postgres", "-d", "marketplace_ai", "-t", "-A", "-c", sql,
    ],
).decode("utf-8", errors="replace")

obj = json.loads(raw.strip())
narr = ((obj.get("dashboard") or {}).get("ad_efficiency") or {}).get("narrative") or {}
out = {
    "headline": narr.get("headline"),
    "summary": narr.get("summary"),
    "metric_claims": [
        {"metric_code": c.get("metric_code"), "value": c.get("value")}
        for c in (narr.get("metric_claims") or [])
    ],
    "action_count": len(obj.get("actions") or []),
    "schema_version": obj.get("schema_version"),
}
path = Path("docs/audits/real-run-2026-07-22-f6aac05/run-194-ad-narrative.json")
path.write_text(json.dumps(out, ensure_ascii=False, indent=2), encoding="utf-8")
print(json.dumps(out, ensure_ascii=False, indent=2)[:3000])
