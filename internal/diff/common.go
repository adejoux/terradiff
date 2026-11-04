package diff

import (
	"sort"
	"strings"
)

// CommonGroup is a set of changes found identically in the same set of
// entities, e.g. a `timeouts` attribute added to every resource.
type CommonGroup struct {
	// Entities are the entities sharing the changes, per kind.
	Entities map[Kind][]string
	// All tells, per kind, whether Entities covers every entity of that
	// kind present in both versions.
	All map[Kind]bool
	// Changes are the shared changes, with Entity cleared.
	Changes []Change
}

type ref struct {
	kind   Kind
	entity string
}

func refKey(r ref) string { return string(r.kind) + "/" + r.entity }

// commonKey identifies a change independently of the entity it applies to.
type commonKey struct {
	severity              Severity
	path, message, detail string
}

// FactorCommon finds the changes that appear identically (same path, same
// message) in at least min entities of one provider, across resources, data
// sources, ephemeral resources, list resources and actions. It returns them
// grouped by set of entities, largest sets first, and the remaining changes.
// The provider configuration, functions and entity additions or removals are
// never factored. min < 2 disables factoring.
func FactorCommon(changes []Change, compared map[Kind][]string, min int) ([]CommonGroup, []Change) {
	if min < 2 {
		return nil, changes
	}
	eligible := func(c Change) bool {
		return c.Kind != KindProvider && c.Kind != KindFunction && !c.IsEntityLevel()
	}

	var order []commonKey
	refs := map[commonKey][]ref{}
	for _, c := range changes {
		if !eligible(c) {
			continue
		}
		k := commonKey{c.Severity, c.Path, c.Message, c.Detail}
		r := ref{c.Kind, c.Entity}
		if n := len(refs[k]); n > 0 && refs[k][n-1] == r {
			continue // same change reported twice for one entity
		}
		if _, ok := refs[k]; !ok {
			order = append(order, k)
		}
		refs[k] = append(refs[k], r)
	}

	// Group the common keys by identical set of entities.
	type group struct {
		sig     string
		size    int
		refs    []ref
		changes []Change
	}
	common := map[commonKey]bool{}
	groups := map[string]*group{}
	var sigs []string
	for _, k := range order {
		rs := refs[k]
		if len(rs) < min {
			continue
		}
		common[k] = true
		sig := sigOf(rs, refKey)
		g, ok := groups[sig]
		if !ok {
			g = &group{sig: sig, size: len(rs), refs: rs}
			groups[sig] = g
			sigs = append(sigs, sig)
		}
	}
	if len(common) == 0 {
		return nil, changes
	}

	var rest []Change
	added := map[commonKey]bool{}
	for _, c := range changes {
		k := commonKey{c.Severity, c.Path, c.Message, c.Detail}
		if !eligible(c) || !common[k] {
			rest = append(rest, c)
			continue
		}
		if added[k] {
			continue
		}
		added[k] = true
		g := groups[sigOf(refs[k], refKey)]
		c.Entity = ""
		g.changes = append(g.changes, c)
	}

	sort.SliceStable(sigs, func(i, j int) bool { return groups[sigs[i]].size > groups[sigs[j]].size })
	out := make([]CommonGroup, 0, len(sigs))
	for _, sig := range sigs {
		g := groups[sig]
		cg := CommonGroup{Entities: map[Kind][]string{}, All: map[Kind]bool{}, Changes: g.changes}
		for _, r := range g.refs {
			cg.Entities[r.kind] = append(cg.Entities[r.kind], r.entity)
		}
		for k, es := range cg.Entities {
			sort.Strings(es)
			cg.All[k] = len(es) == len(compared[k])
		}
		sort.SliceStable(cg.Changes, func(i, j int) bool { return cg.Changes[i].Path < cg.Changes[j].Path })
		out = append(out, cg)
	}
	return out, rest
}

func sigOf[T any](items []T, key func(T) string) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = key(it)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
