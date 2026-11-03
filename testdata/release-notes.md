# example/demo v1.0.0 → v2.0.0

| Summary | Count |
| :--- | ---: |
| ⚠️ Breaking changes | 11 |
| New resources | 1 |
| New data sources | 1 |
| New ephemeral resources | 1 |
| New functions | 1 |
| Removed resources | 1 |
| Removed functions | 1 |
| Deprecations | 2 |
| Enhancements | 11 |
| Other changes | 4 |

## ⚠️ Breaking changes

### Resources

- **`demo_old_thing`**: Removed.
- **`demo_server`**
  - Removed block `backup`.
  - Attribute `cpu_count` type changed from `number` to `string`.
  - Block `disk` nesting changed from `list` to `set`.
  - Block `disk` now accepts at most 2 items (was at most 4 items).
  - Added required attribute `image` (`string`): Image to boot from.
  - `labels` changed from an attribute to a block: use `labels { ... }` instead of `labels = ...`.
  - Attribute `name` is now required (was optional).
  - Attribute `password` is now write-only: its value is no longer stored in state and cannot be referenced.

### Functions

- **`old_func`**: Removed.
- **`parse_id`**: Return type changed from `object({ name = string, region = string })` to `object({ name = string, project = string, region = string })`.

## New features

### New resources

- **`demo_disk`**: Manages a block storage disk.

### New data sources

- **`demo_images`**: Lists available images.

### New ephemeral resources

- **`demo_token`**: Generates a short-lived API token.

### New functions

- `provider::demo::format_id(region string, name string) string`: Builds an ID from its parts.

## Deprecations

### Resources

- **`demo_server`**: Attribute `legacy_flag` is deprecated: Use `feature_flags` instead.
- **`demo_volume`**: This resource is deprecated: Use demo_disk instead.

## Enhancements

### Provider configuration

- Added optional attribute `retries` (`number`): Number of retries on HTTP 5xx errors.

### Resources

- **`demo_server`**
  - Now supports resource identity (`id`, `region`).
  - Added optional attribute `description` (`string`): Free-form description.
  - Added optional attribute `disk.encrypted` (`bool`, computed when not set).
  - Added read-only attribute `ip_address` (`string`).
  - Added optional attribute `network.security_group_ids` (`set(string)`).
  - Attribute `size` is now optional (was required).
  - Added optional attribute `timeouts.delete` (`string`).

### Data sources

- **`demo_server`**
  - Attribute `id` is now optional (was required).
  - Attribute `name` can now be set (optional, was read-only).

### Functions

- **`parse_id`**: Added variadic parameter `options` (`string`).

## Other changes

### Resources

- **`demo_server`**
  - Schema version changed from 0 to 1: existing state is upgraded automatically on the next plan.
  - Attribute `password` is now sensitive: its value is hidden in plan output.
  - Attribute `region` is no longer computed by the provider: it is only set when configured.
  - Attribute `tags` is now computed by the provider when not set.
