package stages

import "testing"

func TestProductionRegistryHas19Stages(t *testing.T) {
	if ProductionRegistry.Count() != 19 {
		t.Fatalf("expected 19 stages, got %d", ProductionRegistry.Count())
	}
	ordered := ProductionRegistry.Ordered()
	if ordered[0].Name != ResolveContext || ordered[1].Name != SealUniverse || ordered[18].Name != PostPublish {
		t.Fatalf("unexpected stage order endpoints: %s .. %s", ordered[0].Name, ordered[18].Name)
	}
	for _, d := range ordered {
		if d.Ordinal < 1 || d.StageVersion == "" || d.ExecutionType == "" {
			t.Fatalf("incomplete definition for %s", d.Name)
		}
		if !d.ImplementedInCurrentBuild {
			t.Fatalf("%s must be implemented", d.Name)
		}
	}
}

func TestTestRegistryIsolated(t *testing.T) {
	if _, ok := ProductionRegistry.Lookup("TEST_STAGE_A"); ok {
		t.Fatal("test stages leaked into production registry")
	}
	if TestRegistry.Count() != 2 {
		t.Fatal("test registry size")
	}
}
