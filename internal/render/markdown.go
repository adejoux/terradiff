// Package render turns a diff report into human-readable release notes.
package render

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/adejoux/terradiff/internal/diff"
)

// Options controls the Markdown output.
type Options struct {
	// Title overrides the generated document title.
	Title string
	// OldVersion and NewVersion are displayed in the title, e.g. v1.2.0.
	OldVersion string
	NewVersion string
	// HeadingLevel is the level of the top heading (1 = "#"). Use 2 or more
	// to embed the output in an existing CHANGELOG.
	HeadingLevel int
	// NoTitle omits the top heading.
	NoTitle bool
	// NoSummary omits the summary table.
	NoSummary bool
	// CommonMin is the number of entities from which an identical change is
	// moved to a "Common changes" sub-section. Values below 2 disable it.
	CommonMin int
	// NoHTML disables HTML in the output: long lists of entities affected
	// by a common change are written as plain text instead of a collapsible
	// <details> block.
	NoHTML bool
}

// Markdown writes the report as Markdown release notes.
func Markdown(w io.Writer, r *diff.Report, opts Options) error {
	m := &md{opts: opts}
	if m.opts.HeadingLevel < 1 {
		m.opts.HeadingLevel = 1
	}
	m.document(r)
	_, err := io.WriteString(w, strings.TrimRight(m.b.String(), "\n")+"\n")
	return err
}

type md struct {
	b    strings.Builder
	opts Options
	// compared lists, per kind, the entities of the current provider
	// present in both versions.
	compared map[diff.Kind][]string
}

func (m *md) heading(level int, text string) {
	if level > 6 {
		level = 6
	}
	m.b.WriteString(strings.Repeat("#", level) + " " + text + "\n\n")
}

func (m *md) line(format string, args ...any) {
	fmt.Fprintf(&m.b, format, args...)
	m.b.WriteString("\n")
}

func (m *md) blank() { m.b.WriteString("\n") }

func (m *md) document(r *diff.Report) {
	level := m.opts.HeadingLevel
	if !m.opts.NoTitle {
		m.heading(level, m.title(r))
		level++
	}

	if len(r.ProvidersAdded) > 0 || len(r.ProvidersRemoved) > 0 {
		for _, p := range r.ProvidersRemoved {
			m.line("- ⚠️ Provider `%s` was removed.", p)
		}
		for _, p := range r.ProvidersAdded {
			m.line("- Provider `%s` was added.", p)
		}
		m.blank()
	}

	if len(r.Providers) == 1 {
		m.provider(level, r.Providers[0], changesOf(r, r.Providers[0]), r.Compared[r.Providers[0]], false)
		return
	}
	for _, p := range r.Providers {
		m.provider(level, p, changesOf(r, p), r.Compared[p], true)
	}
}

func (m *md) title(r *diff.Report) string {
	if m.opts.Title != "" {
		return m.opts.Title
	}
	name := "Provider"
	if len(r.Providers) == 1 {
		name = shortAddress(r.Providers[0])
	}
	switch {
	case m.opts.OldVersion != "" && m.opts.NewVersion != "":
		return fmt.Sprintf("%s %s → %s", name, m.opts.OldVersion, m.opts.NewVersion)
	case m.opts.NewVersion != "":
		return fmt.Sprintf("%s %s", name, m.opts.NewVersion)
	}
	return name + " schema changes"
}

func (m *md) provider(level int, name string, changes []diff.Change, compared map[diff.Kind][]string, withHeading bool) {
	if withHeading {
		m.heading(level, "`"+name+"`")
		level++
	}
	if len(changes) == 0 {
		m.line("No schema changes.")
		m.blank()
		return
	}
	// Changes shared by many entities are shown once, in a "Common
	// changes" sub-section at the top of their section, and counted once.
	m.compared = compared
	common, rest := diff.FactorCommon(changes, compared, m.opts.CommonMin)
	if !m.opts.NoSummary {
		counted := rest
		for _, g := range common {
			counted = append(counted, g.Changes...)
		}
		m.summary(counted)
	}

	// Breaking changes, all kinds together, entity removals first.
	m.section(level, "⚠️ Breaking changes", common, rest, func(c diff.Change) bool { return c.Severity == diff.Breaking })

	// New entities.
	if nc := filter(changes, func(c diff.Change) bool { return c.IsEntityLevel() && c.Op == diff.Added }); len(nc) > 0 {
		m.heading(level, "New features")
		for _, k := range diff.Kinds {
			kc := filter(nc, func(c diff.Change) bool { return c.Kind == k })
			if len(kc) == 0 {
				continue
			}
			m.heading(level+1, "New "+lowerFirst(k.Plural()))
			for _, c := range kc {
				text := "**`" + c.Entity + "`**"
				if k == diff.KindFunction {
					text = c.Message
				}
				if c.Detail != "" {
					text += ": " + escape(c.Detail)
				}
				m.line("- %s", text)
			}
			m.blank()
		}
	}

	sections := []struct {
		title string
		sev   diff.Severity
	}{
		{"Deprecations", diff.Deprecation},
		{"Enhancements", diff.Enhancement},
		{"Other changes", diff.Note},
		{"Documentation", diff.Docs},
	}
	for _, s := range sections {
		m.section(level, s.title, common, rest, func(c diff.Change) bool {
			return c.Severity == s.sev && !(c.IsEntityLevel() && c.Op == diff.Added)
		})
	}
}

// section renders the common and per-entity changes selected by keep under
// a heading, if there are any.
func (m *md) section(level int, title string, common []diff.CommonGroup, rest []diff.Change, keep func(diff.Change) bool) {
	type sharedLines struct {
		scope scope
		lines []string
	}
	var shared []sharedLines
	for _, g := range common {
		if cs := filter(g.Changes, keep); len(cs) > 0 {
			shared = append(shared, sharedLines{m.scopeOf(g), mergeSimilar(cs)})
		}
	}
	own := filter(rest, keep)
	if len(shared) == 0 && len(own) == 0 {
		return
	}
	m.heading(level, title)
	if len(shared) > 0 {
		m.heading(level+1, "Common changes")
		for _, s := range shared {
			m.line("**%s**", s.scope.label)
			m.blank()
			for _, l := range s.lines {
				m.line("- %s", l)
			}
			m.blank()
			m.affected(s.scope)
		}
	}
	m.byKind(level+1, own)
}

// Thresholds of the "Common changes" labels.
const (
	// maxExceptions is the number of unaffected entities of a kind up to
	// which the label reads "all resources except ...".
	maxExceptions = 3
	// maxInlineNames is the number of affected entities of a kind listed in
	// the label; above it, the label only gives their number.
	maxInlineNames = 5
)

// scope describes the entities affected by a group of common changes.
type scope struct {
	// label is the human description: "All resources except `a`", "7
	// resources", "`a`, `b` and `c` (resources)".
	label string
	// hidden lists, per kind, the entities counted but not named in the
	// label.
	hidden []hiddenNames
}

type hiddenNames struct {
	kind  diff.Kind
	names []string
}

func (m *md) scopeOf(g diff.CommonGroup) scope {
	var sc scope
	var parts, allPlurals []string
	allOnly := true
	for _, k := range diff.Kinds {
		es := g.Entities[k]
		if len(es) == 0 {
			continue
		}
		plural := lowerFirst(k.Plural())
		if g.All[k] && len(es) > 1 {
			parts = append(parts, "all "+plural)
			allPlurals = append(allPlurals, plural)
			continue
		}
		allOnly = false
		if missing := exceptions(m.compared[k], es); len(missing) > 0 && len(missing) <= maxExceptions && len(es) > len(missing) && len(es) > maxInlineNames {
			parts = append(parts, "all "+plural+" except "+enumerate(codes(missing)))
			continue
		}
		switch {
		case len(es) == 1:
			parts = append(parts, codes(es)[0]+" ("+k.Singular()+")")
		case len(es) <= maxInlineNames:
			parts = append(parts, enumerate(codes(es))+" ("+plural+")")
		default:
			parts = append(parts, fmt.Sprintf("%d %s", len(es), plural))
			sc.hidden = append(sc.hidden, hiddenNames{k, es})
		}
	}
	label := joinParts(parts)
	if allOnly {
		label = "all " + enumerate(allPlurals)
	}
	sc.label = strings.ToUpper(label[:1]) + label[1:]
	return sc
}

// affected writes the entities counted but not named in a scope label, as
// a collapsible block (or plain text with NoHTML).
func (m *md) affected(sc scope) {
	if len(sc.hidden) == 0 {
		return
	}
	var kinds []string
	for _, h := range sc.hidden {
		kinds = append(kinds, lowerFirst(h.kind.Plural()))
	}
	what := "Affected " + enumerate(kinds)
	if m.opts.NoHTML {
		for _, h := range sc.hidden {
			m.line("Affected %s: %s.", lowerFirst(h.kind.Plural()), strings.Join(codes(h.names), ", "))
			m.blank()
		}
		return
	}
	m.line("<details><summary>%s</summary>", what)
	m.blank()
	for _, h := range sc.hidden {
		if len(sc.hidden) > 1 {
			m.line("%s: %s", h.kind.Plural(), strings.Join(codes(h.names), ", "))
		} else {
			m.line("%s", strings.Join(codes(h.names), ", "))
		}
		m.blank()
	}
	m.line("</details>")
	m.blank()
}

// exceptions returns the entities of all that are not in affected.
func exceptions(all, affected []string) []string {
	in := make(map[string]bool, len(affected))
	for _, e := range affected {
		in[e] = true
	}
	var out []string
	for _, e := range all {
		if !in[e] {
			out = append(out, e)
		}
	}
	sort.Strings(out)
	return out
}

func codes(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "`" + n + "`"
	}
	return out
}

// joinParts joins the per-kind parts of a label, with a comma before the
// last "and" when a part already contains one ("all resources except `a`
// and `b`, and all data sources").
func joinParts(parts []string) string {
	if len(parts) < 2 {
		return strings.Join(parts, "")
	}
	for _, p := range parts {
		if strings.Contains(p, " and ") {
			return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
		}
	}
	return enumerate(parts)
}

func (m *md) summary(changes []diff.Change) {
	type row struct {
		label string
		n     int
	}
	var rows []row
	if n := count(changes, func(c diff.Change) bool { return c.Severity == diff.Breaking }); n > 0 {
		rows = append(rows, row{"⚠️ Breaking changes", n})
	}
	for _, k := range diff.Kinds {
		if n := count(changes, func(c diff.Change) bool { return c.IsEntityLevel() && c.Op == diff.Added && c.Kind == k }); n > 0 {
			rows = append(rows, row{"New " + lowerFirst(k.Plural()), n})
		}
	}
	for _, k := range diff.Kinds {
		if n := count(changes, func(c diff.Change) bool { return c.IsEntityLevel() && c.Op == diff.Removed && c.Kind == k }); n > 0 {
			rows = append(rows, row{"Removed " + lowerFirst(k.Plural()), n})
		}
	}
	for _, s := range []struct {
		label string
		sev   diff.Severity
	}{{"Deprecations", diff.Deprecation}, {"Enhancements", diff.Enhancement}, {"Other changes", diff.Note}, {"Documentation updates", diff.Docs}} {
		if n := count(changes, func(c diff.Change) bool { return c.Severity == s.sev && !c.IsEntityLevel() }); n > 0 {
			rows = append(rows, row{s.label, n})
		}
	}
	m.line("| Summary | Count |")
	m.line("| :--- | ---: |")
	for _, r := range rows {
		m.line("| %s | %d |", r.label, r.n)
	}
	m.blank()
}

// byKind renders changes grouped by kind (sub-heading) then by entity.
func (m *md) byKind(level int, changes []diff.Change) {
	for _, k := range diff.Kinds {
		kc := filter(changes, func(c diff.Change) bool { return c.Kind == k })
		if len(kc) == 0 {
			continue
		}
		m.heading(level, k.Plural())
		if k == diff.KindProvider {
			for _, c := range kc {
				m.line("- %s", changeText(c))
			}
			m.blank()
			continue
		}
		for _, group := range groupByEntity(kc) {
			entity := "**`" + group[0].Entity + "`**"
			lines := mergeSimilar(group)
			if len(lines) == 1 {
				m.line("- %s: %s", entity, lines[0])
				continue
			}
			m.line("- %s", entity)
			for _, l := range lines {
				m.line("  - %s", l)
			}
		}
		m.blank()
	}
}

// mergeThreshold is the number of identical attribute changes in one entity
// from which they are rendered as a single line.
const mergeThreshold = 3

// mergeSimilar renders the changes of one entity, merging attribute changes
// that only differ by their path ("Attributes `a`, `b` and `c` are now
// computed...") so long lists of repetitive changes stay readable. A merged
// line takes the position of the first change it replaces.
func mergeSimilar(changes []diff.Change) []string {
	type key struct{ group, detail string }
	members := map[key][]diff.Change{}
	for _, c := range changes {
		if c.Group != "" && c.Path != "" {
			k := key{c.Group, c.Detail}
			members[k] = append(members[k], c)
		}
	}
	var lines []string
	done := map[key]bool{}
	for _, c := range changes {
		k := key{c.Group, c.Detail}
		if c.Group == "" || c.Path == "" || len(members[k]) < mergeThreshold {
			lines = append(lines, changeText(c))
			continue
		}
		if done[k] {
			continue
		}
		done[k] = true
		paths := make([]string, 0, len(members[k]))
		for _, mc := range members[k] {
			paths = append(paths, "`"+mc.Path+"`")
		}
		text := "Attributes " + enumerate(paths) + " " + c.Group
		if c.Detail != "" {
			text += ": " + escape(c.Detail)
		} else {
			text += "."
		}
		lines = append(lines, text)
	}
	return lines
}

// enumerate joins items as "a, b and c".
func enumerate(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

func changeText(c diff.Change) string {
	if c.Detail == "" {
		return c.Message
	}
	return strings.TrimSuffix(c.Message, ".") + ": " + escape(c.Detail)
}

func groupByEntity(changes []diff.Change) [][]diff.Change {
	var groups [][]diff.Change
	for _, c := range changes {
		if n := len(groups); n > 0 && groups[n-1][0].Entity == c.Entity {
			groups[n-1] = append(groups[n-1], c)
			continue
		}
		groups = append(groups, []diff.Change{c})
	}
	return groups
}

func changesOf(r *diff.Report, provider string) []diff.Change {
	return filter(r.Changes, func(c diff.Change) bool { return c.Provider == provider })
}

func filter(cs []diff.Change, keep func(diff.Change) bool) []diff.Change {
	var out []diff.Change
	for _, c := range cs {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

func count(cs []diff.Change, keep func(diff.Change) bool) int {
	return len(filter(cs, keep))
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

// shortAddress turns registry.terraform.io/hashicorp/aws into hashicorp/aws.
func shortAddress(addr string) string {
	parts := strings.Split(addr, "/")
	if len(parts) == 3 {
		return parts[1] + "/" + parts[2]
	}
	return addr
}

// escape neutralises characters of provider descriptions that would break
// the Markdown layout (HTML tags, table pipes), leaving code spans alone.
func escape(s string) string {
	var b strings.Builder
	inCode := false
	for _, r := range s {
		if r == '`' {
			inCode = !inCode
			b.WriteRune(r)
			continue
		}
		if !inCode {
			switch r {
			case '<':
				b.WriteString("&lt;")
				continue
			case '>':
				b.WriteString("&gt;")
				continue
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}
