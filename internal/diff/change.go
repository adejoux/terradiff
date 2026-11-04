// Package diff compares two provider schemas and produces a list of
// classified, human-readable changes.
package diff

// Severity classifies the impact of a change for provider users.
type Severity string

const (
	// Breaking changes may require users to update their configuration.
	Breaking Severity = "breaking"
	// Deprecation flags something that still works but will go away.
	Deprecation Severity = "deprecation"
	// Enhancement is a backward compatible addition or relaxation.
	Enhancement Severity = "enhancement"
	// Note is a behavioural change worth knowing that is not breaking.
	Note Severity = "note"
	// Docs is a documentation-only change (descriptions).
	Docs Severity = "docs"
)

// Severities lists all severities from the most to the least important.
var Severities = []Severity{Breaking, Deprecation, Enhancement, Note, Docs}

// Rank returns the position of the severity in Severities (lower is more
// important).
func (s Severity) Rank() int {
	for i, v := range Severities {
		if v == s {
			return i
		}
	}
	return len(Severities)
}

// Kind is the kind of provider object a change applies to.
type Kind string

const (
	KindProvider          Kind = "provider"
	KindResource          Kind = "resource"
	KindDataSource        Kind = "data_source"
	KindEphemeralResource Kind = "ephemeral_resource"
	KindListResource      Kind = "list_resource"
	KindAction            Kind = "action"
	KindFunction          Kind = "function"
)

// Kinds lists all kinds in display order.
var Kinds = []Kind{
	KindProvider,
	KindResource,
	KindDataSource,
	KindEphemeralResource,
	KindListResource,
	KindAction,
	KindFunction,
}

// Rank returns the display position of the kind.
func (k Kind) Rank() int {
	for i, v := range Kinds {
		if v == k {
			return i
		}
	}
	return len(Kinds)
}

// Singular is the human name of the kind ("data source").
func (k Kind) Singular() string {
	switch k {
	case KindProvider:
		return "provider configuration"
	case KindResource:
		return "resource"
	case KindDataSource:
		return "data source"
	case KindEphemeralResource:
		return "ephemeral resource"
	case KindListResource:
		return "list resource"
	case KindAction:
		return "action"
	case KindFunction:
		return "function"
	}
	return string(k)
}

// Plural is the human plural name of the kind ("Data sources").
func (k Kind) Plural() string {
	switch k {
	case KindProvider:
		return "Provider configuration"
	case KindResource:
		return "Resources"
	case KindDataSource:
		return "Data sources"
	case KindEphemeralResource:
		return "Ephemeral resources"
	case KindListResource:
		return "List resources"
	case KindAction:
		return "Actions"
	case KindFunction:
		return "Functions"
	}
	return string(k)
}

// Op is what happened to the element.
type Op string

const (
	Added   Op = "added"
	Removed Op = "removed"
	Changed Op = "changed"
)

// Change is a single difference between the two schemas.
type Change struct {
	// Provider is the provider address, e.g. registry.terraform.io/hashicorp/aws.
	Provider string `json:"provider"`
	// Kind of object the change belongs to.
	Kind Kind `json:"kind"`
	// Entity is the resource, data source, function... name. Empty for the
	// provider configuration.
	Entity string `json:"entity,omitempty"`
	// Path is the dotted path of the attribute or block inside the entity.
	// Empty when the change applies to the entity itself.
	Path string `json:"path,omitempty"`
	// Op is the type of change.
	Op Op `json:"op"`
	// Severity is the impact of the change.
	Severity Severity `json:"severity"`
	// Message is a short Markdown sentence describing the change.
	Message string `json:"message"`
	// Detail is optional extra context, typically the first sentence of the
	// description of a new element.
	Detail string `json:"detail,omitempty"`
	// Group, when set, is the plural form of the change for attributes
	// ("are now computed by the provider when not set"). The renderer uses
	// it to merge identical changes of one entity into a single line.
	Group string `json:"-"`
}

// IsEntityLevel reports whether the change is the addition or removal of a
// whole entity (resource, data source, function...).
func (c Change) IsEntityLevel() bool {
	return c.Path == "" && (c.Op == Added || c.Op == Removed) && c.Kind != KindProvider
}

// Report is the result of comparing two schema documents.
type Report struct {
	// ProvidersAdded are provider addresses only present in the new document.
	ProvidersAdded []string `json:"providers_added,omitempty"`
	// ProvidersRemoved are provider addresses only present in the old document.
	ProvidersRemoved []string `json:"providers_removed,omitempty"`
	// Providers are the provider addresses that were compared.
	Providers []string `json:"providers"`
	// Changes are all the differences found, in display order.
	Changes []Change `json:"changes"`
	// Compared lists, per provider and kind, the entities present in both
	// versions. It tells whether a change applies to all of them.
	Compared map[string]map[Kind][]string `json:"-"`
}

// HasBreaking reports whether the report contains breaking changes.
func (r *Report) HasBreaking() bool {
	for _, c := range r.Changes {
		if c.Severity == Breaking {
			return true
		}
	}
	return len(r.ProvidersRemoved) > 0
}

// Count returns the number of changes with the given severity.
func (r *Report) Count(s Severity) int {
	n := 0
	for _, c := range r.Changes {
		if c.Severity == s {
			n++
		}
	}
	return n
}
