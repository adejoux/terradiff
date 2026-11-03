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
