# hashicorp/random v3.1.0 → v3.6.3

| Summary | Count |
| :--- | ---: |
| New resources | 1 |
| Deprecations | 2 |
| Enhancements | 3 |
| Other changes | 20 |

## New features

### New resources

- **`random_bytes`**: The resource `random_bytes` generates random bytes that are intended to be used as a secret, or key.

## Deprecations

### Resources

- **`random_password`**: Attribute `number` is deprecated: Use `numeric` instead.
- **`random_string`**: Attribute `number` is deprecated: Use `numeric` instead.

## Enhancements

### Resources

- **`random_password`**
  - Added read-only attribute `bcrypt_hash` (`string`, sensitive): A bcrypt hash of the generated random string.
  - Added optional attribute `numeric` (`bool`, computed when not set): Include numeric characters in the result.
- **`random_string`**: Added optional attribute `numeric` (`bool`, computed when not set): Include numeric characters in the result.

## Other changes

### Resources

- **`random_password`**
  - Schema version changed from 0 to 3: existing state is upgraded automatically on the next plan.
  - Attributes `lower`, `min_lower`, `min_numeric`, `min_special`, `min_upper`, `number`, `special` and `upper` are now computed by the provider when not set.
- **`random_pet`**
  - Attribute `length` is now computed by the provider when not set.
  - Attribute `separator` is now computed by the provider when not set.
- **`random_string`**
  - Schema version changed from 1 to 2: existing state is upgraded automatically on the next plan.
  - Attributes `lower`, `min_lower`, `min_numeric`, `min_special`, `min_upper`, `number`, `special` and `upper` are now computed by the provider when not set.
