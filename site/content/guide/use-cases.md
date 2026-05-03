---
title: Use cases
description: "Worked examples: knowledge bases, task trackers, CRM directories, config-as-data."
weight: 3
---

These examples show concrete repo structures and the etch commands that drive them. Each one follows the [agent home directory]({{< ref "/guide/home-directories" >}}) pattern.

## Knowledge base / daily journal

A repo of markdown files with YAML frontmatter for metadata and sections for prose.

```
journal/
  2026-05-01.md
  2026-05-02.md
notes/
  project-alpha.md
  meeting-kickoff.md
```

Typical etch operations:

```sh
# Create today's journal entry
etch create journal/2026-05-02.md

# Set frontmatter metadata
etch set journal/2026-05-02.md mood good
etch set journal/2026-05-02.md tags --json '["work","planning"]'

# Append to a section
etch section append journal/2026-05-02.md "## Log" <<'EOF'
- 10:00 Kicked off project alpha
- 11:30 Reviewed draft proposal
EOF

# Replace a summary
etch section replace notes/project-alpha.md "## Status" <<'EOF'
On track. Next milestone: May 15.
EOF
```

## Task tracker

Markdown files as tasks, with frontmatter for structured status and inline fields for in-context metadata.

```
tasks/
  onboarding.md
  review-proposal.md
  ship-v2.md
```

Each file has frontmatter like:

```yaml
---
status: open
priority: high
assigned: alice
tags: [launch, blocking]
---
```

Typical etch operations:

```sh
# Update task status
etch set tasks/onboarding.md status complete

# Snooze with an inline field
etch set tasks/review-proposal.md snoozed "2026-05-10" --task "Review draft"

# Batch update in a script
etch run - <<'SCRIPT'
set tasks/ship-v2.md status in-progress
set tasks/ship-v2.md started "2026-05-02"
delete tasks/ship-v2.md blocked_by
SCRIPT

# Close/open task checkboxes
etch task close tasks/onboarding.md "Set up dev environment"
etch task open tasks/onboarding.md "Schedule intro meetings"

# Add a new task item
etch task add tasks/ship-v2.md "Write migration guide" --section "## Remaining"
```

## People / CRM directory

One markdown file per person with structured metadata in frontmatter and an append-only interaction log.

```
people/
  alice.md
  bob.md
  carol.md
```

Each file looks like:

```yaml
---
name: Alice Chen
role: Engineering Lead
company: Acme Corp
tags: [partner, technical]
last_contact: "2026-04-20"
---

## Notes

Key decision-maker for the integration project.

## Log

- 2026-04-20: Discussed API timeline
- 2026-04-15: Intro call
```

Typical etch operations:

```sh
# Update contact date
etch set people/alice.md last_contact "2026-05-02"

# Add a tag
etch add people/alice.md tags '"vip"'

# Append to the interaction log
etch section append people/alice.md "## Log" <<'EOF'
- 2026-05-02: Reviewed proposal, agreed on scope
EOF

# Batch: update metadata and log together
etch run - <<'SCRIPT'
set people/alice.md last_contact "2026-05-02"
add people/alice.md tags '"active-deal"'
section append people/alice.md "## Log" <<EOF
- 2026-05-02: Signed off on phase 1
EOF
SCRIPT
```

## Config-as-data

YAML or JSON files that represent system state, tracked in git for auditability.

```
config/
  agents.yaml
  schedules.json
  feature-flags.json
events.jsonl
```

Typical etch operations:

```sh
# Toggle a feature flag
etch set config/feature-flags.json dark_mode --json true

# Update agent configuration
etch set config/agents.yaml assistant.model "claude-sonnet-4-6"
etch set config/agents.yaml assistant.temperature --json 0.7

# Set multiple values at once
etch set config/schedules.json daily_digest=09:00 weekly_report=monday

# Append an event record
etch append events.jsonl '{"kind":"deploy","version":"2.1.0","at":"2026-05-02T14:00:00Z"}'

# CSV tracking table
etch table row append config/inventory.csv '{"sku":"A1","status":"active","owner":"ops"}'
etch table set config/inventory.csv sku=A1,status retired
```
