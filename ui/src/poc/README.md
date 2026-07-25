# Build-UX PoCs

Three directions for "build the job", per the discussion on `feat/build-ux-poc`.
Mock data only, no API. Routes are mounted **outside the AuthGuard** so they can
be reviewed without a running Core — delete them with the branch.

```
npm run dev   →   http://localhost:3000/poc
```

## The shared part (the real deliverable)

The layouts differ; the foundation under them does not. `params/registry.tsx`
is a table of input types, each declaring everything the app knows about it:

| | |
|---|---|
| `Input` | the control a runner fills in |
| `Config` | the type-specific half of the schema editor |
| `validate` | the same rule Core applies to an API payload |
| `yamlKeys` | what it serialises to the job definition |
| `toEnv` | how the value reaches the container |
| `seed` | the spec an author gets when they add one |

Nothing else in the app enumerates types — not the form renderer, the builder,
the validator, the YAML writer, or the env preview. **Adding an input type is
one registry entry plus a control.** That is the answer to "Jenkins with 100500
plugins": the extension point is a data structure, not a plugin host.

Ten types ship in the PoC: the eight from PRODUCT.md §5 plus `slider` and
`file`. `slider` exists specifically to demonstrate the cost of a new type — it
is ~15 lines and reuses `NumberConfig` wholesale.

## The three layouts

### A — Inline (`/poc/inline`)

Triggering is the primary act of a job page, so it lives there, always visible.
"Edit inputs" flips the same list into edit mode in place; collapsed rows keep
rendering the real control, greyed out, so the schema list never stops looking
like the form.

- **For:** zero navigation, one mental model, the job page stays the hub.
- **Against:** the run form competes for width with build history; a 20-param
  schema makes an already-long page longer.

### B — Builder (`/poc/builder`)

Running and designing are different jobs, so each gets a whole page. Run mode
is one focused column with a sticky action bar and **rerun-with** — prefill
every input from a previous build. Design mode is the classic three-pane form
builder: palette, canvas, inspector.

- **For:** scales to large schemas; rerun-with is a genuinely missing feature;
  the run page is linkable and shareable.
- **Against:** a route away from the job page, and two modes to keep coherent.

### C — Split (`/poc/split`)

No modes. One surface, permanently split: the form on the left, its
consequences on the right. Hovering a field highlights its lines in the YAML
you would commit, its line in the resolved `docker run` env, or its key in the
equivalent `curl`. Authoring is a gear icon per field, not a separate mode.

- **For:** strongest GitOps story — the file and the run are always both
  visible; the API-parity claim from PRODUCT.md §5 becomes something you can
  *see* rather than something we assert.
- **Against:** needs the width; the right pane is dead weight on a laptop
  unless it collapses.

### D — Workbench (`/poc/workbench`) ← current direction

B's shape, reworked from the feedback on it. Three columns in both modes, each
with a fixed role:

| column | editing | running |
|---|---|---|
| **left** — menu | the configure panel for the selected parameter | **start from**: job defaults, or prefill the whole form from a past build |
| **centre** — main | the parameter list, live; click one to configure, drag to reorder | the form to fill in |
| **right** — context | the YAML you would commit — copy **or download** — highlighting the selected parameter | `docker run` / `curl`, switchable |

What changed from B:

- **The left type palette is gone.** It duplicated the type switcher already
  present in the configure panel. "Add input" appends a text parameter and you
  retype it there, so a type is chosen in exactly one location.
- **The configure panel moved from the right column to the left.** The
  parameter list and the panel behave as they did in B; only the column
  changed.
- **The YAML took over the right column** full-height, with C's line
  highlighting, instead of being buried under the panel — plus a `.yaml`
  download next to copy.

## Ideas worth keeping whichever layout wins

- **Defaults are set with the real control.** Authors pick a default date in
  the date picker, not by typing `2026-07-25` into a text box. It doubles as a
  live preview of the field.
- **`name` and `env` derive from the label** until the author unlinks them, so
  the common case is one field of typing rather than three.
- **Errors stay quiet until first submit.** A form that shouts before you have
  touched it reads as broken.
- **The YAML pane is read-only.** Git is the source of truth; the panel's job
  is to produce a diff, not to own the file (PRODUCT.md §6).
