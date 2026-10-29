package evidence

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

func lineageFingerprint(entity EntityRef, path string, refs []string, formulaKey, formulaVer string) string {
	return canonicaljson.Hash(map[string]any{
		"entity": entity, "semantic_path": path, "fact_refs": refs,
		"formula_key": formulaKey, "formula_version": formulaVer,
	})
}

func discoveryFingerprints(m discovery.FactHandleMapV1) map[string]discovery.HandleEntry {
	out := map[string]discovery.HandleEntry{}
	for _, e := range m.Entries {
		fk, fv := "", ""
		if e.Derivation != nil {
			fk, fv = e.Derivation.FormulaKey, e.Derivation.FormulaVersion
		}
		out[lineageFingerprint(e.Entity, e.SemanticPath, e.FactRefs, fk, fv)] = e
	}
	return out
}

func assignHandles(items []EvidenceItem, results []MaterializationResult, disc discovery.FactHandleMapV1) (UnifiedHandleRegistryV1, []EvidenceItem, error) {
	fps := discoveryFingerprints(disc)
	reg := UnifiedHandleRegistryV1{SchemaVersion: FactHandleRegistryV1, RegistryVersion: FactHandleRegistryV1, Entries: []UnifiedHandle{}}
	usedDisc := map[string]bool{}
	for _, e := range disc.Entries {
		reg.Entries = append(reg.Entries, UnifiedHandle{
			Handle: e.Handle, Origin: "DISCOVERY", Entity: e.Entity, SemanticPath: e.SemanticPath, FactRefs: e.FactRefs,
		})
		if e.Derivation != nil {
			reg.Entries[len(reg.Entries)-1].FormulaKey = e.Derivation.FormulaKey
			reg.Entries[len(reg.Entries)-1].FormulaVersion = e.Derivation.FormulaVersion
			reg.Entries[len(reg.Entries)-1].InputFactRefs = e.Derivation.InputFactRefs
		}
		usedDisc[e.Handle] = true
	}
	ef := 0
	outItems := make([]EvidenceItem, len(items))
	copy(outItems, items)
	for i, res := range results {
		handles := []string{}
		for _, f := range res.Facts {
			fk, fv := "", ""
			var inRefs []string
			if f.Derivation != nil {
				fk, fv = f.Derivation.FormulaKey, f.Derivation.FormulaVersion
				inRefs = f.Derivation.InputFactRefs
			}
			if len(f.FactRefs) == 0 && f.Derivation == nil {
				return reg, nil, Integrity("handle without lineage")
			}
			fp := lineageFingerprint(res.Request.Subject, f.SemanticPath, f.FactRefs, fk, fv)
			h := ""
			if e, ok := fps[fp]; ok {
				h = e.Handle
			} else {
				ef++
				h = fmt.Sprintf("ef_%04d", ef)
			}
			handles = append(handles, h)
			reg.Entries = append(reg.Entries, UnifiedHandle{
				Handle: h, Origin: "EVIDENCE", EvidenceID: outItems[i].EvidenceID, Entity: res.Request.Subject,
				SemanticPath: f.SemanticPath, FactRefs: f.FactRefs, FormulaKey: fk, FormulaVersion: fv, InputFactRefs: inRefs,
			})
		}
		outItems[i].FactHandles = handles
	}
	return reg, outItems, nil
}
