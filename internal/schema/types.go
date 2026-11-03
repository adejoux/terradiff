package schema

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

// TypeString renders a cty type, as encoded in the schema JSON, using the
// Terraform type constraint syntax: string, list(string),
// map(object({ name = string })), and so on.
func TypeString(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	return typeString(v)
}

func typeString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		if len(t) == 0 {
			return "unknown"
		}
		kind, _ := t[0].(string)
		switch kind {
		case "list", "set", "map":
			if len(t) < 2 {
				return kind
			}
			return kind + "(" + typeString(t[1]) + ")"
		case "object":
			if len(t) < 2 {
				return "object({})"
			}
			attrs, _ := t[1].(map[string]any)
			optional := map[string]bool{}
			if len(t) > 2 {
				if opts, ok := t[2].([]any); ok {
					for _, o := range opts {
						if s, ok := o.(string); ok {
							optional[s] = true
						}
					}
				}
			}
			keys := make([]string, 0, len(attrs))
			for k := range attrs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			parts := make([]string, 0, len(keys))
			for _, k := range keys {
				s := typeString(attrs[k])
				if optional[k] {
					s = "optional(" + s + ")"
				}
				parts = append(parts, k+" = "+s)
			}
			if len(parts) == 0 {
				return "object({})"
			}
			return "object({ " + strings.Join(parts, ", ") + " })"
		case "tuple":
			if len(t) < 2 {
				return "tuple([])"
			}
			elems, _ := t[1].([]any)
			parts := make([]string, 0, len(elems))
			for _, e := range elems {
				parts = append(parts, typeString(e))
			}
			return "tuple([" + strings.Join(parts, ", ") + "])"
		}
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// ShortTypeString is like TypeString but collapses long object and tuple
// types so they stay readable inline.
func ShortTypeString(raw json.RawMessage, max int) string {
	s := TypeString(raw)
	if max <= 0 || len(s) <= max {
		return s
	}
	return collapse(s)
}

// collapse replaces the content of object({...}) and tuple([...]) by an
// ellipsis, keeping the outer collection wrappers.
func collapse(s string) string {
	for _, open := range []string{"object({", "tuple(["} {
		if i := strings.Index(s, open); i >= 0 {
			j := matching(s, i+len(open)-1)
			if j < 0 {
				return s
			}
			return s[:i] + open + "…" + s[j:]
		}
	}
	return s
}

// matching returns the index of the bracket closing the one at index i.
func matching(s string, i int) int {
	depth := 0
	for k := i; k < len(s); k++ {
		switch s[k] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth == 0 {
				return k
			}
		}
	}
	return -1
}

// TypesEqual reports whether two encoded cty types are semantically equal.
func TypesEqual(a, b json.RawMessage) bool {
	return TypeString(a) == TypeString(b)
}
