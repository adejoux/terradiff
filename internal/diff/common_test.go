package diff

import "testing"

func TestFactorCommon(t *testing.T) {
	ch := func(kind Kind, entity, path string, sev Severity) Change {
		return Change{Kind: kind, Entity: entity, Path: path, Op: Changed, Severity: sev, Message: "Attribute `" + path + "` changed."}
	}
	changes := []Change{
		ch(KindResource, "a", "region", Breaking),
		ch(KindResource, "a", "tags", Enhancement),
		ch(KindResource, "b", "region", Breaking),
		ch(KindResource, "b", "tags", Enhancement),
		ch(KindResource, "c", "only_c", Note),
		{Kind: KindResource, Entity: "d", Op: Removed, Severity: Breaking, Message: "Removed."},
		ch(KindDataSource, "a", "region", Breaking),
	}
	compared := map[Kind][]string{KindResource: {"a", "b", "c"}, KindDataSource: {"a"}}

	groups, rest := FactorCommon(changes, compared, 3)
	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1: %+v", len(groups), groups)
	}
	g := groups[0]
	if len(g.Changes) != 1 || g.Changes[0].Path != "region" || g.Changes[0].Entity != "" {
		t.Errorf("unexpected common changes: %+v", g.Changes)
	}
	if g.All[KindResource] || !g.All[KindDataSource] {
		t.Errorf("All = %v, want resources partial and data sources complete", g.All)
	}
	if len(rest) != 4 {
		t.Errorf("got %d remaining changes, want 4 (2 tags, only_c, removal)", len(rest))
	}

	if groups, rest := FactorCommon(changes, compared, 0); groups != nil || len(rest) != len(changes) {
		t.Error("min 0 must disable factoring")
	}
}
