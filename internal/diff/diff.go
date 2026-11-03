package diff

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/adejoux/terradiff/internal/schema"
)

// Options tunes the comparison.
type Options struct {
	// Descriptions includes documentation-only changes (descriptions,
	// summaries) in the report.
	Descriptions bool
	// Provider restricts the comparison to a single provider address. It may
	// be the full address (registry.terraform.io/hashicorp/aws) or a suffix
	// (hashicorp/aws, aws).
	Provider string
}

// Compare compares two schema documents.
func Compare(oldDoc, newDoc *schema.Document, opts Options) (*Report, error) {
	r := &Report{}
	pairs, err := matchProviders(oldDoc, newDoc, opts.Provider, r)
	if err != nil {
		return nil, err
	}
	for _, p := range pairs {
		r.Providers = append(r.Providers, p.name)
		d := &differ{opts: opts, provider: p.name}
		if p.oldName != p.name {
			d.add(Change{
				Kind:     KindProvider,
				Op:       Changed,
				Severity: Breaking,
				Message:  fmt.Sprintf("Provider address changed from `%s` to `%s`.", p.oldName, p.name),
			})
		}
		d.providerSchema(p.old, p.new)
		r.Changes = append(r.Changes, d.changes...)
	}
	sortChanges(r.Changes)
	return r, nil
}

type providerPair struct {
	name, oldName string
	old, new      *schema.ProviderSchema
}

func matchProviders(oldDoc, newDoc *schema.Document, filter string, r *Report) ([]providerPair, error) {
	oldNames := filterNames(oldDoc.ProviderNames(), filter)
	newNames := filterNames(newDoc.ProviderNames(), filter)

	if len(oldNames) == 0 && len(newNames) == 0 {
		if filter != "" {
			return nil, fmt.Errorf("no provider matching %q in either document", filter)
		}
		return nil, fmt.Errorf("both documents contain no provider")
	}

	// A single provider on each side is always compared, even if its
	// address changed (e.g. a namespace move or a private registry mirror).
	if len(oldNames) == 1 && len(newNames) == 1 {
		return []providerPair{{
			name:    newNames[0],
			oldName: oldNames[0],
			old:     oldDoc.ProviderSchemas[oldNames[0]],
			new:     newDoc.ProviderSchemas[newNames[0]],
		}}, nil
	}

	inNew := map[string]bool{}
	for _, n := range newNames {
		inNew[n] = true
	}
	var pairs []providerPair
	for _, n := range oldNames {
		if inNew[n] {
			pairs = append(pairs, providerPair{name: n, oldName: n, old: oldDoc.ProviderSchemas[n], new: newDoc.ProviderSchemas[n]})
			delete(inNew, n)
		} else {
			r.ProvidersRemoved = append(r.ProvidersRemoved, n)
		}
	}
	for _, n := range newNames {
		if inNew[n] {
			r.ProvidersAdded = append(r.ProvidersAdded, n)
		}
	}
	return pairs, nil
}

func filterNames(names []string, filter string) []string {
	if filter == "" {
		return names
	}
	var out []string
	for _, n := range names {
		if n == filter || strings.HasSuffix(n, "/"+filter) {
			out = append(out, n)
		}
	}
	return out
}

func sortChanges(cs []Change) {
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if a.Provider != b.Provider {
			return a.Provider < b.Provider
		}
		if a.Kind != b.Kind {
			return a.Kind.Rank() < b.Kind.Rank()
		}
		if a.Entity != b.Entity {
			return a.Entity < b.Entity
		}
		// Entity-level changes first, then by path.
		if (a.Path == "") != (b.Path == "") {
			return a.Path == ""
		}
		return a.Path < b.Path
	})
}

type differ struct {
	opts     Options
	provider string
	changes  []Change

	// current entity
	kind   Kind
	entity string
}

func (d *differ) add(c Change) {
	c.Provider = d.provider
	if c.Kind == "" {
		c.Kind = d.kind
		c.Entity = d.entity
	}
	if c.Severity == Docs && !d.opts.Descriptions {
		return
	}
	d.changes = append(d.changes, c)
}

func (d *differ) providerSchema(o, n *schema.ProviderSchema) {
	if o == nil {
		o = &schema.ProviderSchema{}
	}
	if n == nil {
		n = &schema.ProviderSchema{}
	}

	d.kind, d.entity = KindProvider, ""
	d.schemaBody(o.Provider, n.Provider)

	d.schemaMap(KindResource, o.ResourceSchemas, n.ResourceSchemas)
	d.schemaMap(KindDataSource, o.DataSourceSchemas, n.DataSourceSchemas)
	d.schemaMap(KindEphemeralResource, o.EphemeralResourceSchemas, n.EphemeralResourceSchemas)
	d.schemaMap(KindListResource, o.ListResourceSchemas, n.ListResourceSchemas)
	d.schemaMap(KindAction, o.ActionSchemas, n.ActionSchemas)
	d.functions(o.Functions, n.Functions)
	d.identities(o.ResourceIdentitySchemas, n.ResourceIdentitySchemas, o.ResourceSchemas, n.ResourceSchemas)
}

func (d *differ) schemaMap(kind Kind, o, n map[string]*schema.Schema) {
	for _, name := range unionKeys(o, n) {
		d.kind, d.entity = kind, name
		os, ns := o[name], n[name]
		switch {
		case os == nil:
			d.add(Change{Op: Added, Severity: Enhancement, Message: "New " + kind.Singular() + ".", Detail: summary(blockDescription(ns))})
		case ns == nil:
			d.add(Change{Op: Removed, Severity: Breaking, Message: "Removed."})
		default:
			d.schemaBody(os, ns)
		}
	}
}

func (d *differ) schemaBody(o, n *schema.Schema) {
	if o == nil && n == nil {
		return
	}
	if o == nil {
		o = &schema.Schema{}
	}
	if n == nil {
		n = &schema.Schema{}
	}
	if o.Version != n.Version && d.kind != KindProvider {
		d.add(Change{
			Op:       Changed,
			Severity: Note,
			Message:  fmt.Sprintf("Schema version changed from %d to %d: existing state is upgraded automatically on the next plan.", o.Version, n.Version),
		})
	}
	ob, nb := o.Block, n.Block
	if ob == nil {
		ob = &schema.Block{}
	}
	if nb == nil {
		nb = &schema.Block{}
	}
	if !ob.Deprecated && nb.Deprecated {
		d.add(Change{Op: Changed, Severity: Deprecation, Message: "This " + d.kind.Singular() + " is deprecated.", Detail: deprecationNote(nb.Description)})
	} else if ob.Deprecated && !nb.Deprecated {
		d.add(Change{Op: Changed, Severity: Note, Message: "No longer deprecated."})
	}
	if ob.Description != nb.Description {
		d.add(Change{Op: Changed, Severity: Docs, Message: "Description updated.", Detail: summary(nb.Description)})
	}
	d.blockContent("", ob, nb)
}

// blockContent compares the attributes and nested blocks of two blocks.
func (d *differ) blockContent(prefix string, o, n *schema.Block) {
	// Detect attributes turned into blocks and vice versa first so they are
	// reported once instead of as a removal plus an addition.
	moved := map[string]bool{}
	for name := range o.Attributes {
		if _, ok := n.Attributes[name]; !ok {
			if _, ok := n.BlockTypes[name]; ok {
				moved[name] = true
				d.add(Change{
					Path: join(prefix, name), Op: Changed, Severity: Breaking,
					Message: fmt.Sprintf("`%s` changed from an attribute to a block: use `%s { ... }` instead of `%s = ...`.", join(prefix, name), name, name),
				})
			}
		}
	}
	for name := range o.BlockTypes {
		if _, ok := n.BlockTypes[name]; !ok {
			if _, ok := n.Attributes[name]; ok {
				moved[name] = true
				d.add(Change{
					Path: join(prefix, name), Op: Changed, Severity: Breaking,
					Message: fmt.Sprintf("`%s` changed from a block to an attribute: use `%s = ...` instead of `%s { ... }`.", join(prefix, name), name, name),
				})
			}
		}
	}

	for _, name := range unionKeys(o.Attributes, n.Attributes) {
		if moved[name] {
			continue
		}
		d.attribute(join(prefix, name), o.Attributes[name], n.Attributes[name])
	}
	for _, name := range unionKeys(o.BlockTypes, n.BlockTypes) {
		if moved[name] {
			continue
		}
		d.nestedBlock(join(prefix, name), o.BlockTypes[name], n.BlockTypes[name])
	}
}

func (d *differ) nestedBlock(path string, o, n *schema.NestedBlock) {
	switch {
	case o == nil:
		sev, req := Enhancement, ""
		if n.MinItems > 0 {
			sev, req = Breaking, "required "
		}
		d.add(Change{
			Path: path, Op: Added, Severity: sev,
			Message: fmt.Sprintf("Added %sblock `%s`%s.", req, path, blockShape(n)),
			Detail:  summary(blockDescription2(n.Block)),
		})
		return
	case n == nil:
		d.add(Change{Path: path, Op: Removed, Severity: Breaking, Message: fmt.Sprintf("Removed block `%s`.", path)})
		return
	}

	if o.NestingMode != n.NestingMode {
		d.add(Change{
			Path: path, Op: Changed, Severity: Breaking,
			Message: fmt.Sprintf("Block `%s` nesting changed from `%s` to `%s`.", path, o.NestingMode, n.NestingMode),
		})
	}
	d.itemLimits("Block", path, o.MinItems, o.MaxItems, n.MinItems, n.MaxItems)

	ob, nb := o.Block, n.Block
	if ob == nil {
		ob = &schema.Block{}
	}
	if nb == nil {
		nb = &schema.Block{}
	}
	if !ob.Deprecated && nb.Deprecated {
		d.add(Change{Path: path, Op: Changed, Severity: Deprecation, Message: fmt.Sprintf("Block `%s` is deprecated.", path), Detail: deprecationNote(nb.Description)})
	} else if ob.Deprecated && !nb.Deprecated {
		d.add(Change{Path: path, Op: Changed, Severity: Note, Message: fmt.Sprintf("Block `%s` is no longer deprecated.", path)})
	}
	if ob.Description != nb.Description {
		d.add(Change{Path: path, Op: Changed, Severity: Docs, Message: fmt.Sprintf("Description of block `%s` updated.", path), Detail: summary(nb.Description)})
	}
	d.blockContent(path, ob, nb)
}

func (d *differ) itemLimits(what, path string, oMin, oMax, nMin, nMax uint64) {
	if oMin != nMin {
		sev := Enhancement
		if nMin > oMin {
			sev = Breaking
		}
		d.add(Change{
			Path: path, Op: Changed, Severity: sev,
			Message: fmt.Sprintf("%s `%s` now requires at least %d item(s) (was %d).", what, path, nMin, oMin),
		})
	}
	if oMax != nMax {
		sev := Enhancement
		if nMax != 0 && (oMax == 0 || nMax < oMax) {
			sev = Breaking
		}
		d.add(Change{
			Path: path, Op: Changed, Severity: sev,
			Message: fmt.Sprintf("%s `%s` now accepts %s (was %s).", what, path, maxLabel(nMax), maxLabel(oMax)),
		})
	}
}

func (d *differ) attribute(path string, o, n *schema.Attribute) {
	switch {
	case o == nil:
		sev, extra := Enhancement, ""
		if n.Required {
			sev = Breaking
		}
		if n.Optional && n.Computed {
			extra = ", computed when not set"
		}
		if n.Sensitive {
			extra += ", sensitive"
		}
		if n.WriteOnly {
			extra += ", write-only"
		}
		if n.Deprecated {
			extra += ", deprecated"
		}
		d.add(Change{
			Path: path, Op: Added, Severity: sev,
			Message: fmt.Sprintf("Added %s attribute `%s` (`%s`%s).", modeLabel(n), path, attrType(n), extra),
			Detail:  summary(n.Description),
		})
		return
	case n == nil:
		d.add(Change{Path: path, Op: Removed, Severity: Breaking, Message: fmt.Sprintf("Removed attribute `%s`.", path), Group: "were removed"})
		return
	}

	d.attrMode(path, o, n)

	// Type.
	switch {
	case o.NestedType == nil && n.NestedType == nil:
		if !schema.TypesEqual(o.Type, n.Type) {
			d.add(Change{
				Path: path, Op: Changed, Severity: Breaking,
				Message: fmt.Sprintf("Attribute `%s` type changed from `%s` to `%s`.", path, attrType(o), attrType(n)),
			})
		}
	case o.NestedType != nil && n.NestedType != nil:
		if o.NestedType.NestingMode != n.NestedType.NestingMode {
			d.add(Change{
				Path: path, Op: Changed, Severity: Breaking,
				Message: fmt.Sprintf("Attribute `%s` type changed from `%s` to `%s`.", path, attrType(o), attrType(n)),
			})
		}
		d.itemLimits("Attribute", path, o.NestedType.MinItems, o.NestedType.MaxItems, n.NestedType.MinItems, n.NestedType.MaxItems)
		for _, name := range unionKeys(o.NestedType.Attributes, n.NestedType.Attributes) {
			d.attribute(join(path, name), o.NestedType.Attributes[name], n.NestedType.Attributes[name])
		}
	default:
		// Switch between a plain type and a nested attribute. The HCL syntax
		// is the same, so compare the resulting types.
		ot, nt := attrType(o), attrType(n)
		if ot != nt {
			d.add(Change{
				Path: path, Op: Changed, Severity: Breaking,
				Message: fmt.Sprintf("Attribute `%s` type changed from `%s` to `%s`.", path, ot, nt),
			})
		}
	}

	if !o.Sensitive && n.Sensitive {
		d.add(Change{Path: path, Op: Changed, Severity: Note, Message: fmt.Sprintf("Attribute `%s` is now sensitive: its value is hidden in plan output.", path), Group: "are now sensitive: their values are hidden in plan output"})
	} else if o.Sensitive && !n.Sensitive {
		d.add(Change{Path: path, Op: Changed, Severity: Note, Message: fmt.Sprintf("Attribute `%s` is no longer sensitive.", path), Group: "are no longer sensitive"})
	}
	if !o.WriteOnly && n.WriteOnly {
		d.add(Change{Path: path, Op: Changed, Severity: Breaking, Message: fmt.Sprintf("Attribute `%s` is now write-only: its value is no longer stored in state and cannot be referenced.", path)})
	} else if o.WriteOnly && !n.WriteOnly {
		d.add(Change{Path: path, Op: Changed, Severity: Note, Message: fmt.Sprintf("Attribute `%s` is no longer write-only: its value is now stored in state.", path)})
	}
	if !o.Deprecated && n.Deprecated {
		d.add(Change{Path: path, Op: Changed, Severity: Deprecation, Message: fmt.Sprintf("Attribute `%s` is deprecated.", path), Detail: deprecationNote(n.Description), Group: "are deprecated"})
	} else if o.Deprecated && !n.Deprecated {
		d.add(Change{Path: path, Op: Changed, Severity: Note, Message: fmt.Sprintf("Attribute `%s` is no longer deprecated.", path), Group: "are no longer deprecated"})
	}
	if o.Description != n.Description {
		d.add(Change{Path: path, Op: Changed, Severity: Docs, Message: fmt.Sprintf("Description of attribute `%s` updated.", path), Detail: summary(n.Description), Group: "have an updated description"})
	}
}

// attrMode reports changes of the required / optional / computed flags.
func (d *differ) attrMode(path string, o, n *schema.Attribute) {
	om, nm := attrMode(o), attrMode(n)
	if om == nm {
		return
	}
	readOnly := func(a *schema.Attribute) bool { return a.Computed && !a.Optional && !a.Required }

	var sev Severity
	var msg, group string
	switch {
	case n.Required && !o.Required:
		sev, msg = Breaking, fmt.Sprintf("Attribute `%s` is now required (was %s).", path, modeLabel(o))
		group = fmt.Sprintf("are now required (was %s)", modeLabel(o))
	case o.Required && !n.Required && !readOnly(n):
		sev, msg = Enhancement, fmt.Sprintf("Attribute `%s` is now %s (was required).", path, modeLabel(n))
		group = fmt.Sprintf("are now %s (was required)", modeLabel(n))
	case readOnly(n):
		sev, msg = Breaking, fmt.Sprintf("Attribute `%s` is now read-only and can no longer be set (was %s).", path, modeLabel(o))
		group = fmt.Sprintf("are now read-only and can no longer be set (was %s)", modeLabel(o))
	case readOnly(o):
		sev, msg = Enhancement, fmt.Sprintf("Attribute `%s` can now be set (%s, was read-only).", path, modeLabel(n))
		group = fmt.Sprintf("can now be set (%s, was read-only)", modeLabel(n))
	case !o.Computed && n.Computed:
		sev, msg = Note, fmt.Sprintf("Attribute `%s` is now computed by the provider when not set.", path)
		group = "are now computed by the provider when not set"
	case o.Computed && !n.Computed:
		sev, msg = Note, fmt.Sprintf("Attribute `%s` is no longer computed by the provider: it is only set when configured.", path)
		group = "are no longer computed by the provider: they are only set when configured"
	default:
		sev, msg = Note, fmt.Sprintf("Attribute `%s` is now %s (was %s).", path, nm, om)
		group = fmt.Sprintf("are now %s (was %s)", nm, om)
	}
	d.add(Change{Path: path, Op: Changed, Severity: sev, Message: msg, Group: group})
}

func (d *differ) functions(o, n map[string]*schema.Function) {
	for _, name := range unionKeys(o, n) {
		d.kind, d.entity = KindFunction, name
		of, nf := o[name], n[name]
		switch {
		case of == nil:
			desc := nf.Summary
			if desc == "" {
				desc = nf.Description
			}
			d.add(Change{Op: Added, Severity: Enhancement, Message: "`" + signature(providerLocalName(d.provider), name, nf) + "`", Detail: summary(desc)})
			continue
		case nf == nil:
			d.add(Change{Op: Removed, Severity: Breaking, Message: "Removed."})
			continue
		}
		d.function(of, nf)
	}
}

func (d *differ) function(o, n *schema.Function) {
	if of, nf := schema.TypeString(o.ReturnType), schema.TypeString(n.ReturnType); of != nf {
		d.add(Change{Op: Changed, Severity: Breaking, Message: fmt.Sprintf("Return type changed from `%s` to `%s`.", of, nf)})
	}

	max := len(o.Parameters)
	if len(n.Parameters) > max {
		max = len(n.Parameters)
	}
	for i := 0; i < max; i++ {
		var op, np *schema.FunctionParameter
		if i < len(o.Parameters) {
			op = o.Parameters[i]
		}
		if i < len(n.Parameters) {
			np = n.Parameters[i]
		}
		d.parameter(fmt.Sprintf("parameter #%d", i+1), op, np, false)
	}
	d.parameter("variadic parameter", o.VariadicParameter, n.VariadicParameter, true)

	if o.DeprecationMessage == "" && n.DeprecationMessage != "" {
		d.add(Change{Op: Changed, Severity: Deprecation, Message: "This function is deprecated.", Detail: summary(n.DeprecationMessage)})
	} else if o.DeprecationMessage != "" && n.DeprecationMessage == "" {
		d.add(Change{Op: Changed, Severity: Note, Message: "No longer deprecated."})
	}
	if o.Summary != n.Summary || o.Description != n.Description {
		desc := n.Summary
		if desc == "" {
			desc = n.Description
		}
		d.add(Change{Op: Changed, Severity: Docs, Message: "Description updated.", Detail: summary(desc)})
	}
}

func (d *differ) parameter(label string, o, n *schema.FunctionParameter, variadic bool) {
	switch {
	case o == nil && n == nil:
		return
	case o == nil:
		sev := Breaking
		if variadic {
			sev = Enhancement
		}
		d.add(Change{
			Path: n.Name, Op: Added, Severity: sev,
			Message: fmt.Sprintf("Added %s `%s` (`%s`).", label, n.Name, schema.TypeString(n.Type)),
			Detail:  summary(n.Description),
		})
		return
	case n == nil:
		d.add(Change{Path: o.Name, Op: Removed, Severity: Breaking, Message: fmt.Sprintf("Removed %s `%s`.", label, o.Name)})
		return
	}
	if o.Name != n.Name {
		d.add(Change{Path: n.Name, Op: Changed, Severity: Note, Message: fmt.Sprintf("%s renamed from `%s` to `%s` (parameters are positional: calls are not affected).", capitalize(label), o.Name, n.Name)})
	}
	if ot, nt := schema.TypeString(o.Type), schema.TypeString(n.Type); ot != nt {
		d.add(Change{Path: n.Name, Op: Changed, Severity: Breaking, Message: fmt.Sprintf("%s `%s` type changed from `%s` to `%s`.", capitalize(label), n.Name, ot, nt)})
	}
	oNull, nNull := o.AllowNullValue || o.IsNullable, n.AllowNullValue || n.IsNullable
	if oNull && !nNull {
		d.add(Change{Path: n.Name, Op: Changed, Severity: Breaking, Message: fmt.Sprintf("%s `%s` no longer accepts `null`.", capitalize(label), n.Name)})
	} else if !oNull && nNull {
		d.add(Change{Path: n.Name, Op: Changed, Severity: Enhancement, Message: fmt.Sprintf("%s `%s` now accepts `null`.", capitalize(label), n.Name)})
	}
	if o.Description != n.Description {
		d.add(Change{Path: n.Name, Op: Changed, Severity: Docs, Message: fmt.Sprintf("Description of %s `%s` updated.", label, n.Name), Detail: summary(n.Description)})
	}
}

// identities compares resource identity schemas. Identity changes are
// reported on the resource they belong to.
func (d *differ) identities(o, n map[string]*schema.IdentitySchema, oldResources, newResources map[string]*schema.Schema) {
	for _, name := range unionKeys(o, n) {
		d.kind, d.entity = KindResource, name
		oi, ni := o[name], n[name]
		switch {
		case oi == nil:
			if _, ok := oldResources[name]; !ok {
				continue // new resource: identity support is implied
			}
			d.add(Change{Path: "", Op: Changed, Severity: Enhancement, Message: "Now supports resource identity" + identityAttrs(ni) + "."})
			continue
		case ni == nil:
			if _, ok := newResources[name]; !ok {
				continue // the resource itself was removed
			}
			d.add(Change{Op: Changed, Severity: Breaking, Message: "No longer supports resource identity."})
			continue
		}
		if oi.Version != ni.Version {
			d.add(Change{Op: Changed, Severity: Note, Message: fmt.Sprintf("Resource identity version changed from %d to %d.", oi.Version, ni.Version)})
		}
		for _, an := range unionKeys(oi.Attributes, ni.Attributes) {
			oa, na := oi.Attributes[an], ni.Attributes[an]
			path := "identity." + an
			switch {
			case oa == nil:
				sev := Enhancement
				if na.RequiredForImport {
					sev = Breaking
				}
				d.add(Change{Path: path, Op: Added, Severity: sev, Message: fmt.Sprintf("Added identity attribute `%s` (`%s`, %s).", an, schema.TypeString(na.Type), importMode(na))})
			case na == nil:
				d.add(Change{Path: path, Op: Removed, Severity: Breaking, Message: fmt.Sprintf("Removed identity attribute `%s`.", an)})
			default:
				if ot, nt := schema.TypeString(oa.Type), schema.TypeString(na.Type); ot != nt {
					d.add(Change{Path: path, Op: Changed, Severity: Breaking, Message: fmt.Sprintf("Identity attribute `%s` type changed from `%s` to `%s`.", an, ot, nt)})
				}
				if om, nm := importMode(oa), importMode(na); om != nm {
					sev := Enhancement
					if na.RequiredForImport {
						sev = Breaking
					}
					d.add(Change{Path: path, Op: Changed, Severity: sev, Message: fmt.Sprintf("Identity attribute `%s` is now %s (was %s).", an, nm, om)})
				}
			}
		}
	}
}

// --- helpers ---------------------------------------------------------------

func unionKeys[V any](a, b map[string]V) []string {
	seen := make(map[string]bool, len(a)+len(b))
	keys := make([]string, 0, len(a)+len(b))
	for k := range a {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for k := range b {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys
}

func join(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func attrMode(a *schema.Attribute) string {
	switch {
	case a.Required:
		return "required"
	case a.Optional && a.Computed:
		return "optional+computed"
	case a.Optional:
		return "optional"
	case a.Computed:
		return "read-only"
	}
	return "unspecified"
}

// modeLabel is the human label of an attribute mode, used in messages.
func modeLabel(a *schema.Attribute) string {
	if a.Optional {
		return "optional"
	}
	return attrMode(a)
}

func attrType(a *schema.Attribute) string {
	if a.NestedType == nil {
		return schema.ShortTypeString(a.Type, 60)
	}
	obj := "object"
	switch a.NestedType.NestingMode {
	case "single", "":
		return obj
	case "list", "set", "map":
		return a.NestedType.NestingMode + "(" + obj + ")"
	}
	return a.NestedType.NestingMode + "(" + obj + ")"
}

func blockShape(b *schema.NestedBlock) string {
	var parts []string
	switch b.NestingMode {
	case "single", "group":
		parts = append(parts, "single")
	case "":
	default:
		parts = append(parts, b.NestingMode)
	}
	if b.MinItems > 0 {
		parts = append(parts, fmt.Sprintf("at least %d", b.MinItems))
	}
	if b.MaxItems > 0 && b.NestingMode != "single" && b.NestingMode != "group" {
		parts = append(parts, fmt.Sprintf("at most %d", b.MaxItems))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func maxLabel(n uint64) string {
	if n == 0 {
		return "an unlimited number of items"
	}
	if n == 1 {
		return "at most 1 item"
	}
	return fmt.Sprintf("at most %d items", n)
}

func blockDescription(s *schema.Schema) string {
	if s == nil {
		return ""
	}
	return blockDescription2(s.Block)
}

func blockDescription2(b *schema.Block) string {
	if b == nil {
		return ""
	}
	return b.Description
}

func importMode(a *schema.IdentityAttribute) string {
	switch {
	case a.RequiredForImport:
		return "required for import"
	case a.OptionalForImport:
		return "optional for import"
	}
	return "informational"
}

func identityAttrs(s *schema.IdentitySchema) string {
	if s == nil || len(s.Attributes) == 0 {
		return ""
	}
	names := unionKeys(s.Attributes, nil)
	for i, n := range names {
		names[i] = "`" + n + "`"
	}
	return " (" + strings.Join(names, ", ") + ")"
}

// providerLocalName returns the default local name of a provider, used in
// function calls (provider::<name>::<function>).
func providerLocalName(addr string) string {
	if i := strings.LastIndex(addr, "/"); i >= 0 {
		return addr[i+1:]
	}
	return addr
}

func signature(local, name string, f *schema.Function) string {
	var params []string
	for _, p := range f.Parameters {
		params = append(params, p.Name+" "+schema.ShortTypeString(p.Type, 40))
	}
	if v := f.VariadicParameter; v != nil {
		params = append(params, "..."+v.Name+" "+schema.ShortTypeString(v.Type, 40))
	}
	return fmt.Sprintf("provider::%s::%s(%s) %s", local, name, strings.Join(params, ", "), schema.ShortTypeString(f.ReturnType, 40))
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// summary returns the first sentence of a description, on one line and
// limited in length, for display next to a change.
func summary(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Keep the first paragraph only.
	if i := strings.Index(s, "\n\n"); i >= 0 {
		s = s[:i]
	}
	s = strings.Join(strings.Fields(s), " ")
	// First sentence: a period followed by a space (avoid cutting "e.g." or
	// version numbers like 1.2).
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '.' && s[i+1] == ' ' && i > 0 && !isAbbrev(s[:i+1]) {
			s = s[:i+1]
			break
		}
	}
	const max = 160
	if r := []rune(s); len(r) > max {
		s = strings.TrimSpace(string(r[:max-1])) + "…"
	}
	return s
}

// deprecationNote extracts the most useful sentence of the description of a
// deprecated element: the one mentioning the deprecation, else the one
// suggesting a replacement ("use X instead"), else the first sentence.
func deprecationNote(desc string) string {
	sentences := splitSentences(strings.Join(strings.Fields(desc), " "))
	for _, words := range [][]string{{"deprecat"}, {"instead", "replaced by", "superseded"}} {
		for _, s := range sentences {
			ls := strings.ToLower(s)
			for _, w := range words {
				if strings.Contains(ls, w) {
					return summary(cleanNote(s))
				}
			}
		}
	}
	return summary(desc)
}

// cleanNote removes admonition prefixes such as "**NOTE**:" or "~> ".
func cleanNote(s string) string {
	s = strings.TrimSpace(s)
	for {
		before := s
		for _, p := range []string{"~>", "->", "!>", "**NOTE**:", "**Note**:", "NOTE:", "Note:", "**Deprecated**:", "Deprecated:"} {
			s = strings.TrimSpace(strings.TrimPrefix(s, p))
		}
		if s == before {
			break
		}
	}
	// "This is deprecated, use X instead." → "Use X instead."
	if rest := redundantDeprecation.ReplaceAllString(s, ""); rest != "" {
		s = rest
	}
	return capitalize(s)
}

var redundantDeprecation = regexp.MustCompile(`(?i)^(this|it)(\s+\w+)?\s+(is|has been|was)\s+(now\s+)?deprecated(\s+and\s+will\s+be\s+removed[^,.;:]*)?[,;:]\s*`)

// splitSentences splits a one-line text on ". " while keeping common
// abbreviations together.
func splitSentences(s string) []string {
	var out []string
	start := 0
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '.' && s[i+1] == ' ' && !isAbbrev(s[start:i+1]) {
			out = append(out, strings.TrimSpace(s[start:i+1]))
			start = i + 2
		}
	}
	if rest := strings.TrimSpace(s[start:]); rest != "" {
		out = append(out, rest)
	}
	return out
}

func isAbbrev(s string) bool {
	for _, a := range []string{"e.g.", "i.e.", "etc.", "vs.", "Ex.", "ex."} {
		if strings.HasSuffix(s, a) {
			return true
		}
	}
	return false
}
