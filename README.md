# terradiff

`terradiff` compares two versions of a Terraform provider schema and writes
human-readable **release notes in Markdown**.

It reads the JSON produced by `terraform providers schema -json` (or
`tofu providers schema -json`), finds every difference between the two
versions, classifies each one by impact and renders them grouped so that a
provider user can see at a glance what they have to change in their
configuration.

## Installation

```sh
go install github.com/adejoux/terradiff@latest
```

## Usage

1. Export the schema of each provider version:

   ```sh
   # in a working directory pinned to the old version
   terraform init && terraform providers schema -json > old.json
   # in a working directory pinned to the new version
   terraform init && terraform providers schema -json > new.json
   ```

   For a provider you are developing, use a `dev_overrides` block in a
   Terraform CLI configuration file to point Terraform at a local build,
   then run `terraform providers schema -json` without `terraform init`.

2. Compare them:

   ```sh
   terradiff --old-version v1.2.0 --new-version v1.3.0 old.json new.json > RELEASE_NOTES.md
   ```

Flags can be placed before or after the files. Use `-` for one of the files
to read it from stdin.

| Flag | Description |
| :--- | :--- |
| `-o FILE` | Write the output to a file instead of stdout. |
| `--format markdown\|json` | Output format. `json` emits the structured list of changes, for scripting. |
| `--old-version`, `--new-version` | Version labels shown in the title. |
| `--title TEXT` | Custom document title. |
| `--heading-level N` | Level of the top heading. Use `2` or more to paste the output into an existing `CHANGELOG.md`. |
| `--no-title` | Omit the top heading. |
| `--no-summary` | Omit the summary table. |
| `--descriptions` | Also report documentation-only changes (descriptions). Off by default. |
| `--provider ADDR` | Only compare one provider when the files contain several (full address or suffix, e.g. `hashicorp/aws`). |
| `--fail-on-breaking` | Exit with code `2` when breaking changes are found (useful in CI). |
| `--version` | Print the terradiff version. |

## Output

The release notes are organised by impact, most important first:

1. **⚠️ Breaking changes**: anything that may require users to change their
   configuration or that can break references (removed resources and
   attributes, new required attributes, type changes, attributes becoming
   read-only or write-only, stricter block limits, attribute ↔ block
   conversions, function signature changes...).
2. **New features**: new resources, data sources, ephemeral resources, list
   resources, actions and provider-defined functions, with the first sentence
   of their description.
3. **Deprecations**, with the replacement hint taken from the description
   when there is one.
4. **Enhancements**: backward compatible additions and relaxations (new
   optional attributes, required attributes becoming optional, resource
   identity support...).
5. **Other changes**: behaviour changes worth knowing that are not breaking
   (schema version bumps, sensitivity, computed flags...).
6. **Documentation** (with `--descriptions` only).

Inside each section, changes are grouped by kind (resources, data sources...)
then by entity. Identical changes on many attributes of the same entity are
merged into a single line.

Example, comparing `hashicorp/random` v3.1.0 and v3.6.3 (full output in
[`testdata/random/release-notes.md`](testdata/random/release-notes.md)):

```markdown
## Deprecations

### Resources

- **`random_password`**: Attribute `number` is deprecated: Use `numeric` instead.
- **`random_string`**: Attribute `number` is deprecated: Use `numeric` instead.

## Enhancements

### Resources

- **`random_password`**
  - Added read-only attribute `bcrypt_hash` (`string`, sensitive): A bcrypt hash of the generated random string.
  - Added optional attribute `numeric` (`bool`, computed when not set): Include numeric characters in the result.

## Other changes

### Resources

- **`random_password`**
  - Schema version changed from 0 to 3: existing state is upgraded automatically on the next plan.
  - Attributes `lower`, `min_lower`, `min_numeric`, `min_special`, `min_upper`, `number`, `special` and `upper` are now computed by the provider when not set.
```

A synthetic example covering most kinds of changes is in
[`testdata/release-notes.md`](testdata/release-notes.md).

## What is compared

- Provider configuration, resources, data sources, ephemeral resources, list
  resources and actions: attributes (including nested attributes of the
  Plugin Framework), nested blocks, types, required / optional / computed
  flags, `sensitive`, `write_only`, `deprecated`, nesting modes, min / max
  items and schema versions.
- Provider-defined functions: parameters, variadic parameter, nullability,
  return type and deprecation.
- Resource identity schemas.

Descriptions are only compared with `--descriptions`.

## Development

```sh
go test ./...
go test . -update   # regenerate the golden files in testdata/
```
