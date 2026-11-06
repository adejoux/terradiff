package render

import (
	"reflect"
	"testing"

	"github.com/adejoux/terradiff/internal/diff"
)

func TestMergeSimilar(t *testing.T) {
	c := func(path, group string) diff.Change {
		return diff.Change{Path: path, Message: "Attribute `" + path + "` changed.", Group: group}
	}
	changes := []diff.Change{
		{Message: "Schema version changed."},
		c("a", "are now computed"),
		c("b", "are now computed"),
		c("x", "are now sensitive"),
		c("c", "are now computed"),
		c("y", "are now sensitive"),
	}
	got := mergeSimilar(changes)
	want := []string{
		"Schema version changed.",
		"Attributes `a`, `b` and `c` are now computed.",
		"Attribute `x` changed.",
		"Attribute `y` changed.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mergeSimilar =\n%q\nwant\n%q", got, want)
	}
}

func TestScopeLabel(t *testing.T) {
	m := &md{compared: map[diff.Kind][]string{
		diff.KindResource:   {"a", "b", "c", "d", "e", "f", "g", "h"},
		diff.KindDataSource: {"x", "y"},
	}}
	tests := []struct {
		entities map[diff.Kind][]string
		all      map[diff.Kind]bool
		want     string
		hidden   int
	}{
		{map[diff.Kind][]string{diff.KindResource: {"a", "b", "c", "d", "e", "f", "g", "h"}, diff.KindDataSource: {"x", "y"}},
			map[diff.Kind]bool{diff.KindResource: true, diff.KindDataSource: true}, "All resources and data sources", 0},
		{map[diff.Kind][]string{diff.KindResource: {"a", "b", "c", "d", "e", "f"}, diff.KindDataSource: {"x", "y"}},
			map[diff.Kind]bool{diff.KindDataSource: true}, "All resources except `g` and `h`, and all data sources", 0},
		{map[diff.Kind][]string{diff.KindResource: {"a", "b", "c"}}, nil, "`a`, `b` and `c` (resources)", 0},
		{map[diff.Kind][]string{diff.KindResource: {"a"}, diff.KindDataSource: {"x"}}, nil, "`a` (resource) and `x` (data source)", 0},
		{map[diff.Kind][]string{diff.KindResource: {"a", "b", "c", "d", "e", "f"}}, nil, "All resources except `g` and `h`", 0},
	}
	for _, tt := range tests {
		sc := m.scopeOf(diff.CommonGroup{Entities: tt.entities, All: tt.all})
		if sc.label != tt.want || len(sc.hidden) != tt.hidden {
			t.Errorf("label = %q (hidden %d), want %q (hidden %d)", sc.label, len(sc.hidden), tt.want, tt.hidden)
		}
	}

	// More than maxInlineNames entities and too many exceptions: counted.
	m.compared[diff.KindResource] = append(m.compared[diff.KindResource], "i", "j")
	sc := m.scopeOf(diff.CommonGroup{Entities: map[diff.Kind][]string{diff.KindResource: {"a", "b", "c", "d", "e", "f"}}})
	if sc.label != "6 resources" || len(sc.hidden) != 1 {
		t.Errorf("label = %q (hidden %d), want \"6 resources\" (hidden 1)", sc.label, len(sc.hidden))
	}
}
