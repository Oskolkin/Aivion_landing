#!/usr/bin/env python3
import json
import pathlib
import sys

sys.stdout.reconfigure(encoding="utf-8")
base = pathlib.Path("debug")
raw = (base / "run23_full_out.txt").read_text(encoding="utf-8", errors="replace")
idx = raw.rfind('\n{"actions"')
if idx < 0:
    idx = raw.find('{"actions"')
else:
    idx += 1
out = json.loads(raw[idx:])
(base / "run23_validated.json").write_text(json.dumps(out, ensure_ascii=False, indent=2), encoding="utf-8")

home = json.loads((base / "_home_pub14.json").read_text(encoding="utf-8"))
acc = home["ai_diagnostics"]["account_summary"]
narr = home.get("seller_pulse", {}).get("narrative") or {}
wc = home["ai_diagnostics"]["what_changed"]
lines = [
    f"HOME publication_id={home.get('publication_id')} status={home.get('dashboard_content_status')}",
    f"HOME health_score={acc.get('health_score')} severity={acc.get('severity')} label={acc.get('health_status_label')}",
    f"HOME levers={len(acc.get('improvement_levers') or [])}",
    f"HOME improvement={(acc.get('improvement_summary') or '')[:220]}",
    f"HOME pulse_summary_len={len(narr.get('summary') or '')}",
    f"HOME pulse_summary={narr.get('summary')}",
    f"HOME notes={len(narr.get('important_notes') or [])}",
]
for n in narr.get("important_notes") or []:
    lines.append(f"  NOTE {n.get('title')} | {(n.get('text') or '')[:160]}")
lines.append(
    "HOME QA="
    + str([(a.get("label"), a.get("href"), a.get("priority")) for a in (narr.get("quick_actions") or [])])
)
lines.append(f"HOME period={home.get('period_context')}")
lines.append(f"HOME wc_main={(wc.get('main_conclusion') or '')[:320]}")
for g in wc.get("groups") or []:
    lines.append(f"  GROUP {g.get('key')} {g.get('label')} n={len(g.get('items') or [])}")
blob = json.dumps(home, ensure_ascii=False)
for term in ["DRR", "CTR", "AI-диагност", "Пульс селлера", "/app/advertising", "/app/products", "Норма 30%", "Подсказка появится", "Статус не определён"]:
    lines.append(f"term {term!r} present={term in blob}")
(base / "home_pub14_evidence.txt").write_text("\n".join(lines), encoding="utf-8")
print("\n".join(lines))
sp = out["dashboard"]["seller_pulse"]["narrative"]
print("AI pulse sentences", (sp.get("summary") or "").count("."), "len", len(sp.get("summary") or ""))
print("AI important_facts", len(sp.get("important_facts") or []))
print("AI levers", len(out["dashboard"]["account_summary"].get("improvement_levers") or []))
print("AI health", out["dashboard"]["account_summary"].get("health_score"), out["dashboard"]["account_summary"].get("severity"))
