package schema

import (
	"encoding/json"
	"testing"
)

func TestTypeString(t *testing.T) {
	tests := map[string]string{
		`"string"`:                                   "string",
		`["list","string"]`:                          "list(string)",
		`["map",["set","number"]]`:                   "map(set(number))",
		`["object",{"b":"bool","a":"string"}]`:       "object({ a = string, b = bool })",
		`["object",{"a":"string","b":"bool"},["b"]]`: "object({ a = string, b = optional(bool) })",
		`["tuple",["string","number"]]`:              "tuple([string, number])",
		`"dynamic"`:                                  "dynamic",
		``:                                           "",
	}
	for in, want := range tests {
		if got := TypeString(json.RawMessage(in)); got != want {
			t.Errorf("TypeString(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestShortTypeString(t *testing.T) {
	in := json.RawMessage(`["list",["object",{"name":"string","value":"string","description":"string"}]]`)
	if got, want := ShortTypeString(in, 20), "list(object({…}))"; got != want {
		t.Errorf("ShortTypeString = %q, want %q", got, want)
	}
	if got := ShortTypeString(json.RawMessage(`["list","string"]`), 20); got != "list(string)" {
		t.Errorf("short types must not be collapsed, got %q", got)
	}
}
