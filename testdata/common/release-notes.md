# example/acme v2.0.0

| Summary | Count |
| :--- | ---: |
| ⚠️ Breaking changes | 2 |
| Deprecations | 1 |
| Enhancements | 1 |
| Other changes | 3 |

## ⚠️ Breaking changes

### Common changes

- **All resources and data sources**: Added required attribute `region` (`string`): Region of the object.

### Resources

- **`acme_d`**: Removed attribute `legacy`.

## Deprecations

### Common changes

- **`acme_a`, `acme_b` and `acme_c` (resources)**: Attribute `labels` is deprecated: Use `tags` instead.

## Enhancements

### Common changes

- **All resources**: Added optional attribute `tags` (`map(string)`): Tags applied to the object.

## Other changes

### Common changes

- **All resources**: Attributes `owner`, `project_id` and `zone` are now computed by the provider when not set.
