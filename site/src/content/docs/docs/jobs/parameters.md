---
title: Parameters
description: Typed inputs that render the run form and reach the command as environment variables.
---

Parameters are the inputs of a job's run form. The panel renders a control for each one, validates
what you enter, and hands every value to the command as an environment variable. No plugin is
involved.

```yaml
parameters:
  - name: regions
    type: multiselect
    label: Regions
    env: REGIONS
    description: Passed to the command as a comma-separated list.
    default: [eu-central, us-east]
    options:
      - { value: eu-central, label: eu-central }
      - { value: us-east, label: us-east }
```

## Keys

| Key | Required | Meaning |
|---|---|---|
| `name` | yes | Identifier, unique within the job. |
| `type` | yes | One of the types below. |
| `label` | no | Text above the control. |
| `env` | no | Environment variable name. Defaults to `name` in upper snake case: `dry_run` → `DRY_RUN`. |
| `description` | no | Help text under the control. |
| `required` | no | The run is refused without a value. |
| `default` | no | Used when the form leaves the value empty. |
| `options` | `select`, `multiselect` | Allowed values; see below. |

## Types

| Type | Control | Value the command gets |
|---|---|---|
| `string` | Text field | The text as typed. |
| `number` | Number field | The number, e.g. `4` or `2.5`. |
| `bool` | Toggle | `true` or `false`. |
| `select` | Dropdown | The chosen option's `value`. |
| `multiselect` | Chips | Chosen values joined with commas: `eu-central,us-east`. |
| `date` | Date picker | `YYYY-MM-DD`. |
| `datetime` | Date and time picker | ISO 8601, e.g. `2026-09-30T09:00:00Z`. |
| `secret` | Masked field | Reserved: secret values are not resolved yet and are not passed to the command. |

## Options

Each option of a `select` or `multiselect` has a `value` and may add:

| Key | Meaning |
|---|---|
| `label` | Text shown instead of the value. |
| `hint` | Secondary text beside the label, e.g. the URL an environment deploys to. |
| `tone` | Colours the option: `success`, `warning` or `danger`. |

A value outside the options is refused, whether it comes from the form or the API.
