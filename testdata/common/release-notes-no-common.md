# example/acme v2.0.0

| Summary | Count |
| :--- | ---: |
| ⚠️ Breaking changes | 7 |
| Deprecations | 3 |
| Enhancements | 4 |
| Other changes | 12 |

## ⚠️ Breaking changes

### Resources

- **`acme_a`**: Added required attribute `region` (`string`): Region of the object.
- **`acme_b`**: Added required attribute `region` (`string`): Region of the object.
- **`acme_c`**: Added required attribute `region` (`string`): Region of the object.
- **`acme_d`**
  - Removed attribute `legacy`.
  - Added required attribute `region` (`string`): Region of the object.

### Data sources

- **`acme_a`**: Added required attribute `region` (`string`): Region of the object.
- **`acme_b`**: Added required attribute `region` (`string`): Region of the object.

## Deprecations

### Resources

- **`acme_a`**: Attribute `labels` is deprecated: Use `tags` instead.
- **`acme_b`**: Attribute `labels` is deprecated: Use `tags` instead.
- **`acme_c`**: Attribute `labels` is deprecated: Use `tags` instead.

## Enhancements

### Resources

- **`acme_a`**: Added optional attribute `tags` (`map(string)`): Tags applied to the object.
- **`acme_b`**: Added optional attribute `tags` (`map(string)`): Tags applied to the object.
- **`acme_c`**: Added optional attribute `tags` (`map(string)`): Tags applied to the object.
- **`acme_d`**: Added optional attribute `tags` (`map(string)`): Tags applied to the object.

## Other changes

### Resources

- **`acme_a`**: Attributes `owner`, `project_id` and `zone` are now computed by the provider when not set.
- **`acme_b`**: Attributes `owner`, `project_id` and `zone` are now computed by the provider when not set.
- **`acme_c`**: Attributes `owner`, `project_id` and `zone` are now computed by the provider when not set.
- **`acme_d`**: Attributes `owner`, `project_id` and `zone` are now computed by the provider when not set.
