#!/usr/bin/env python3
import json
import sys
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")
h = json.loads(Path("debug/_home_pub14b.json").read_text(encoding="utf-8"))
acc = h["ai_diagnostics"]["account_summary"]
narr = (h.get("seller_pulse") or {}).get("narrative") or {}
wc = h["ai_diagnostics"]["what_changed"]
lines = []
lines.append(f"pub={h.get('publication_id')} status={h.get('dashboard_content_status')}")
lines.append(
    f"score={acc.get('health_score')} sev={acc.get('severity')} "
    f"label={acc.get('health_status_label')} code={acc.get('health_status_code')} "
    f"explain={acc.get('health_status_explanation')}"
)
imp = acc.get("improvement_summary") or ""
lines.append(f"levers={len(acc.get('improvement_levers') or [])} imp={imp[:220]}")
for L in acc.get("improvement_levers") or []:
    lines.append(f"  LEVER {L}")
lines.append(f"period={h.get('period_context')}")
lines.append(f"pulse_len={len(narr.get('summary') or '')}")
lines.append(f"pulse={narr.get('summary')}")
lines.append(f"notes={len(narr.get('important_notes') or [])}")
for n in narr.get("important_notes") or []:
    lines.append(f"  NOTE {n.get('title')} | {(n.get('text') or '')[:160]}")
qa = [(a.get("label"), a.get("href"), a.get("priority"), a.get("action_id")) for a in (narr.get("quick_actions") or [])]
lines.append(f"qa={qa}")
lines.append(
    f"wc_status={wc.get('status')} msg={wc.get('message')} "
    f"groups={len(wc.get('groups') or [])} main_len={len(wc.get('main_conclusion') or '')}"
)
lines.append(f"wc_main={(wc.get('main_conclusion') or '')[:400]}")
for g in wc.get("groups") or []:
    lines.append(f"  G {g.get('key')} {g.get('label')} n={len(g.get('items') or [])}")
    for it in (g.get("items") or [])[:2]:
        lines.append(f"    - {it.get('title') or it.get('label')}: {(it.get('text') or it.get('description') or '')[:140]}")
for kpi in (h.get("seller_pulse") or {}).get("kpis") or []:
    lines.append(f"kpi {kpi.get('key')} label={kpi.get('label')} value={kpi.get('value')}")
blob = json.dumps(h, ensure_ascii=False)
for term in [
    "/app/advertising",
    "/app/products",
    "/app/stocks-replenishment",
    "/app/actions",
    "Статус не определён",
    "Подсказка появится",
    "Норма 30%",
    '"CTR"',
    "Кликабельность",
    "ДРР",
    "Пульс продавца",
    "ИИ-диагностика",
]:
    lines.append(f"term {term!r} => {term in blob}")
Path("debug/home_pub14b_evidence.txt").write_text("\n".join(lines), encoding="utf-8")
print("\n".join(lines))
