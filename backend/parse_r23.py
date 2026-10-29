#!/usr/bin/env python3
import json
import sys
from pathlib import Path

sys.stdout.reconfigure(encoding="utf-8")
text = Path("run23_full_out.txt").read_text(encoding="utf-16")
needle = '{"actions"'
last = text.rfind(needle)
print("last", last)
out = json.loads(text[last:])
Path("run23_validated.json").write_text(json.dumps(out, ensure_ascii=False, indent=2), encoding="utf-8")
wc = out["dashboard"]["what_changed"]
print("main_len", len(wc.get("main_conclusion") or ""))
print("main", (wc.get("main_conclusion") or "")[:500])
for g in wc.get("groups") or []:
    print("GROUP", g.get("key"), "|", g.get("label"), "| items", len(g.get("items") or []))
    for it in (g.get("items") or [])[:3]:
        print(" ", {k: it.get(k) for k in ("title", "label", "text", "signal_id", "primary_section", "status") if it.get(k) is not None})
acc = out["dashboard"]["account_summary"]
print("score", acc.get("health_score"), "sev", acc.get("severity"))
print("levers", len(acc.get("improvement_levers") or []))
for L in acc.get("improvement_levers") or []:
    print(" LEVER", L)
sp = out["dashboard"]["seller_pulse"]["narrative"]
print("important_facts", len(sp.get("important_facts") or []))
for f in (sp.get("important_facts") or [])[:5]:
    print(" FACT", f)
print("pulse_len", len(sp.get("summary") or ""))
print("pulse", sp.get("summary"))
