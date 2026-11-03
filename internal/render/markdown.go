// Package render turns a diff report into human-readable release notes.
package render

import (
	"fmt"
	"io"
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
		m.provider(level, r.Providers[0], changesOf(r, r.Providers[0]), false)
		return
	}
	for _, p := range r.Providers {
		m.provider(level, p, changesOf(r, p), true)
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

func (m *md) provider(level int, name string, changes []diff.Change, withHeading bool) {
	if withHeading {
		m.heading(level, "`"+name+"`")
		level++
	}
	if len(changes) == 0 {
		m.line("No schema changes.")
		m.blank()
		return
	}
	if !m.opts.NoSummary {
		m.summary(changes)
	}

	// Breaking changes, all kinds together, entity removals first.
	if bc := filter(changes, func(c diff.Change) bool { return c.Severity == diff.Breaking }); len(bc) > 0 {
		m.heading(level, "⚠️ Breaking changes")
		m.byKind(level+1, bc)
	}

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
		sc := filter(changes, func(c diff.Change) bool {
			return c.Severity == s.sev && !(c.IsEntityLevel() && c.Op == diff.Added)
		})
		if len(sc) == 0 {
			continue
		}
		m.heading(level, s.title)
		m.byKind(level+1, sc)
	}
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
