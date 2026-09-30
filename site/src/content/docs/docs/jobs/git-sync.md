---
title: Syncing from Git
description: How definitions move from YAML files into Lute, and back.
---

Git is the source of truth for job definitions. Core reads the YAML files in `JOB_DEFS_DIR` on
startup and whenever someone presses **Sync from Git** in the panel.

## What a sync does

- **Writes a definition only when its file changed.** Edits made in the panel stay until someone
  changes the file in Git.
- **Keeps definitions that are missing from Git.** Turn on **Prune** in Settings to delete them
  instead.
- **Refuses what it cannot run.** A document without a `name`, a parameter of an unknown type, or a
  `select` without options fails the sync with the document number.

## Drift

Anything that differs from Git is flagged in the panel: the **Differs from Git** filter on the Jobs
page lists it. Open the definition to see what changed.

## Exporting back to YAML

To keep a panel edit, export it and commit the file:

- **Export config** on the Jobs page downloads every definition, as one YAML file or a zip with a
  file per job.
- The **Definition** tab of a job shows its YAML with **copy** and **.yaml** buttons.

Commit the file to the directory core syncs from, and the next sync takes it as the new truth.
