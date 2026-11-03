// Package schema models the JSON document produced by
// `terraform providers schema -json` (and `tofu providers schema -json`).
//
// Only the fields that matter for comparing two provider versions are
// modelled. Unknown fields are ignored so the tool keeps working when
// Terraform adds new keys to the format.
package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

// Document is the top-level output of `terraform providers schema -json`.
type Document struct {
	FormatVersion   string                     `json:"format_version"`
	ProviderSchemas map[string]*ProviderSchema `json:"provider_schemas"`
}

// ProviderSchema describes everything a single provider exposes.
type ProviderSchema struct {
	Provider                 *Schema                    `json:"provider,omitempty"`
	ResourceSchemas          map[string]*Schema         `json:"resource_schemas,omitempty"`
	DataSourceSchemas        map[string]*Schema         `json:"data_source_schemas,omitempty"`
	EphemeralResourceSchemas map[string]*Schema         `json:"ephemeral_resource_schemas,omitempty"`
	ListResourceSchemas      map[string]*Schema         `json:"list_resource_schemas,omitempty"`
	ActionSchemas            map[string]*Schema         `json:"action_schemas,omitempty"`
	Functions                map[string]*Function       `json:"functions,omitempty"`
	ResourceIdentitySchemas  map[string]*IdentitySchema `json:"resource_identity_schemas,omitempty"`
}

// Schema is the schema of a provider configuration, resource, data source,
// ephemeral resource, list resource or action.
type Schema struct {
	Version int64  `json:"version"`
	Block   *Block `json:"block,omitempty"`
}

// Block is a configuration block with attributes and nested blocks.
type Block struct {
	Attributes      map[string]*Attribute   `json:"attributes,omitempty"`
	BlockTypes      map[string]*NestedBlock `json:"block_types,omitempty"`
	Description     string                  `json:"description,omitempty"`
	DescriptionKind string                  `json:"description_kind,omitempty"`
	Deprecated      bool                    `json:"deprecated,omitempty"`
}

// Attribute is a single attribute of a block.
type Attribute struct {
	Type            json.RawMessage `json:"type,omitempty"`
	NestedType      *NestedType     `json:"nested_type,omitempty"`
	Description     string          `json:"description,omitempty"`
	DescriptionKind string          `json:"description_kind,omitempty"`
	Required        bool            `json:"required,omitempty"`
	Optional        bool            `json:"optional,omitempty"`
	Computed        bool            `json:"computed,omitempty"`
	Sensitive       bool            `json:"sensitive,omitempty"`
	Deprecated      bool            `json:"deprecated,omitempty"`
	WriteOnly       bool            `json:"write_only,omitempty"`
}

// NestedType describes a nested attribute (Plugin Framework / protocol 6).
type NestedType struct {
	Attributes  map[string]*Attribute `json:"attributes,omitempty"`
	NestingMode string                `json:"nesting_mode,omitempty"`
	MinItems    uint64                `json:"min_items,omitempty"`
	MaxItems    uint64                `json:"max_items,omitempty"`
}

// NestedBlock is a block nested inside another block.
type NestedBlock struct {
	NestingMode string `json:"nesting_mode,omitempty"`
	Block       *Block `json:"block,omitempty"`
	MinItems    uint64 `json:"min_items,omitempty"`
	MaxItems    uint64 `json:"max_items,omitempty"`
}

// Function is a provider-defined function.
type Function struct {
	Description        string               `json:"description,omitempty"`
	Summary            string               `json:"summary,omitempty"`
	DeprecationMessage string               `json:"deprecation_message,omitempty"`
	ReturnType         json.RawMessage      `json:"return_type,omitempty"`
	Parameters         []*FunctionParameter `json:"parameters,omitempty"`
	VariadicParameter  *FunctionParameter   `json:"variadic_parameter,omitempty"`
}

// FunctionParameter is a positional or variadic function parameter.
type FunctionParameter struct {
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	Type           json.RawMessage `json:"type,omitempty"`
	IsNullable     bool            `json:"is_nullable,omitempty"`
	AllowNullValue bool            `json:"allow_null_value,omitempty"`
}

// IdentitySchema is the resource identity schema of a managed resource.
type IdentitySchema struct {
	Version    int64                         `json:"version"`
	Attributes map[string]*IdentityAttribute `json:"attributes,omitempty"`
}

// IdentityAttribute is one attribute of a resource identity.
type IdentityAttribute struct {
	Type              json.RawMessage `json:"type,omitempty"`
	Description       string          `json:"description,omitempty"`
	RequiredForImport bool            `json:"required_for_import,omitempty"`
	OptionalForImport bool            `json:"optional_for_import,omitempty"`
}

// Load reads and decodes a schema document from a file. "-" means stdin.
func Load(path string) (*Document, error) {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	return Decode(r)
}

// Decode decodes a schema document.
func Decode(r io.Reader) (*Document, error) {
	var doc Document
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decoding provider schema JSON: %w", err)
	}
	if doc.ProviderSchemas == nil {
		return nil, fmt.Errorf("no \"provider_schemas\" key found: is this the output of `terraform providers schema -json`?")
	}
	return &doc, nil
}

// ProviderNames returns the provider addresses in the document, sorted.
func (d *Document) ProviderNames() []string {
	names := make([]string, 0, len(d.ProviderSchemas))
	for n := range d.ProviderSchemas {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
