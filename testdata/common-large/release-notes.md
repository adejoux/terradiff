# example/acme v2.0.0

| Summary | Count |
| :--- | ---: |
| ⚠️ Breaking changes | 2 |
| Deprecations | 1 |
| Enhancements | 1 |
| Other changes | 2 |

## ⚠️ Breaking changes

### Common changes

**All resources except `acme_dns_record` and `acme_iam_role`, and all data sources**

- Added required attribute `region` (`string`): Region of the object.

### Resources

- **`acme_dns_record`**: Removed attribute `legacy`.

## Deprecations

### Common changes

**6 resources**

- Attribute `labels` is deprecated: Use `tags` instead.

<details><summary>Affected resources</summary>

`acme_bucket`, `acme_database`, `acme_instance`, `acme_lb`, `acme_network`, `acme_subnet`

</details>

## Enhancements

### Common changes

**`acme_bucket`, `acme_instance` and `acme_volume` (resources)**

- Added optional attribute `tags` (`map(string)`): Tags applied to the object.

## Other changes

### Common changes

**All resources except `acme_dns_record` and `acme_iam_role`**

- Attribute `owner` is now computed by the provider when not set.
- Attribute `zone` is now computed by the provider when not set.
