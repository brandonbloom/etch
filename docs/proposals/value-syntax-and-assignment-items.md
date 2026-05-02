---
status: implemented
depends_on: []
---

# Value Syntax and Assignment Items

## Summary

Replace JSON-if-valid value inference with one explicit value-input rule across
structured mutations:

- Literal value operands are strings.
- JSON value operands are opt-in.
- Field assignment items use HTTPie-style `selector=value` for strings and
  `selector:=json` for JSON values.

The first version can land for raw JSON and YAML files because those formats
already have singular selector semantics. Markdown frontmatter can adopt the
same value syntax as structured data; Markdown body fields can adopt it later,
after Markdown field and addressing semantics are settled.

## Candidate Commands

```sh
etch <set|append|add|remove> <path> <selector> <value>
etch <set|append|add|remove> <path> <selector> --json <json-value>

etch set <path.json> <selector=value>...
etch set <path.json> <selector:=json>...

etch set <path.yaml> <selector=value>...
etch set <path.yaml> <selector:=json>...
```

Examples:

```sh
etch set cache/state.json last_ingestion 2026-05-02
etch set cache/state.json count --json 12
etch append cache/state.json events --json '{"kind":"prompt"}'
etch set cache/state.json last_ingestion=2026-05-02 count:=12
etch set cache/state.json stats.daily.prompts:=4 tags:='["agent","active"]'
etch set cache/state.yaml owner=Brandon enabled:=true
```

## Value Semantics

- Positional `<value>` operands are literal strings by default for structured
  value-bearing commands.
- Positional `--json <json-value>` operands are parsed as strict JSON for
  structured value-bearing commands.
- `--json` is a command-local flag that consumes exactly one following token as
  the JSON value. It is not a mode switch for later operands.
- `selector=value` sets the selector to the literal string `value`.
- `selector:=json` sets the selector to the parsed strict JSON value.
- Etch no longer parses values as JSON-if-valid. `set file.json count 12` sets
  `"12"`; `set file.json count --json 12` and `set file.json count:=12` set
  `12`.
- The JSON form accepts strict JSON only. JavaScript object-literal shorthand,
  YAML snippets, and shell-like interpolation are not accepted.
- JSON is the structured value language for JSON, YAML, and frontmatter. YAML
  can represent these values, but Etch should not infer YAML scalar types from
  unmarked strings.
- YAML adapters must render string values so they round-trip as strings. For
  example, `set file.yaml enabled=true` should write a string value such as
  `enabled: "true"`, not a boolean-shaped scalar such as `enabled: true`.
- YAML-specific value snippets, such as tags, anchors, aliases, and block
  scalars, are deferred until there is a concrete source-preservation need.

## Assignment Semantics

- Assignment items are accepted only by `set` surfaces, including porcelain
  `set` and format-explicit plumbing such as `json set` and `yaml set`.
- Assignment items are not accepted by `append`, `add`, `remove`, or `delete`.
  Those commands use positional values and explicit `--json` values.
- The selector side uses the same singular selector syntax as ordinary `set`.
  Bare member names remain shorthand for top-level fields.
- Multiple assignment items in one invocation are applied to the same file.
- `selector=` sets the selected value to the empty string.
- `selector:=null` sets the selected value to JSON null for JSON and YAML.
  Frontmatter adopts the same rule when it adopts this value syntax.
- Command forms are exclusive. After `set <path>`, callers must use either the
  positional single-value form or the assignment-item form, not both.
- Assignment items are the multi-field form. Positional `set <path> <selector>
  <value>` remains the single-field form.
- Duplicate assignment targets in the same invocation are errors rather than
  last-write-wins.
- If any assignment item fails validation or mutation, the whole invocation
  fails and produces no commit.
- If Markdown address flags later compose with assignment items, a single
  invocation has one file and one address. All assignment items apply at that
  address. To mutate fields in multiple sections, tasks, list items, or blocks,
  callers should use multiple script lines and `etch run`.

## Script And Agent Ergonomics

- Generated scripts should choose the operator from the source value type:
  literal text uses `=`, typed structured values use JSON encoding plus `:=`.
- Agents should not pre-quote JSON strings by hand. A typed string value should
  be JSON-encoded and written with `:=`, such as `title:="Hello"`.
- Human-authored scripts can keep common string updates concise with `=`, while
  still making booleans, numbers, arrays, objects, and null explicit with
  `:=`.
- The script tokenizer remains shell-like but has no expansions. Values are
  still ordinary tokens, so spaces and shell-sensitive characters require the
  existing quoting rules.

## Rationale

Single-line multi-field updates are common when updating JSON watermarks, YAML
state files, frontmatter, and Dataview inline fields. A script already provides
batching, but assignment items make the common "set several fields in this one
file" shape compact while keeping Etch's one-invocation, one-plan, one-commit
model.

JSON-if-valid is concise, but it makes value type depend on parser inference:
`12`, `true`, `null`, and `"12"` are all special while `hello` is not. That is
easy for both humans and agents to misremember, and it conflicts with the
assignment-item model where `=` and `:=` make the type explicit.

Raw YAML parsing was also considered for YAML files. It is ergonomic for some
hand-written configuration edits, but YAML scalar inference makes values such
as `yes`, `on`, `null`, dates, and numbers depend on YAML version and parser
behavior. Strict JSON is a safer common structured value language and is a
valid subset for JSON-shaped YAML values.

Raw JSON and YAML can use assignment items before Markdown fields because they
only need the existing structured selector engine. Keeping future Markdown
address flags global avoids order-sensitive command lines such as "these two
fields in this task, then that field in another task." Those belong in `etch
run` where each line has one address.

## Impact

Spec:

- Replace JSON-if-valid value parsing with explicit string and JSON modes.
- Add `--json` for positional value-bearing structured commands.
- Add assignment-item parsing for porcelain and plumbing JSON/YAML `set`.
- Define `=` literal-string and `:=` strict-JSON value semantics.
- Define exclusive positional and assignment-item command forms.
- Define duplicate-target and partial-failure behavior.
- Reserve invocation-wide address flag semantics for later Markdown field
  composition.

Docs:

- Update value docs and examples to explain strings by default and explicit
  JSON values.
- Add examples for multi-field JSON and YAML object updates.
- Explain when to use assignment items versus `etch run`.
- Add generated-script guidance for templating literal strings versus typed
  values.

Code:

- Remove JSON-if-valid parsing from JSON, YAML, and frontmatter value-bearing
  commands.
- Add `--json` value-mode decoding for positional value commands.
- Add assignment-item token parsing for `set` commands before command-specific
  decoding.
- Extend JSON and YAML `set` decoders to emit multiple normalized operations
  from one invocation.
- Add fixtures for string default values, explicit JSON values, literal values
  that look like JSON, values containing `=`, duplicate targets, mixed
  valid/invalid assignments, mixed-form rejection, and compatibility with
  single-field `set`.

Verification:

- Assert `set file.json count 12` writes the string `"12"`.
- Assert `set file.json count --json 12` and `set file.json count:=12` write
  the number `12`.
- Assert `append file.json events --json '{"kind":"prompt"}'` appends an
  object and `append file.json events '{"kind":"prompt"}'` appends a string.
- Assert `append file.json events:='{"kind":"prompt"}'` fails because
  assignment items are `set`-only.
- Assert `set file.yaml enabled=true` writes the string `"true"` and
  `set file.yaml enabled:=true` writes the boolean `true`.
- Assert YAML string output for `enabled=true` cannot round-trip as a boolean.
- Assert mixed positional and assignment forms fail before planning.
- Assert scripts preserve the same value semantics as direct argv.

## Deferred

- Assignment syntax for deletion. `delete` remains the removal operation; a
  deletion sigil would add parser complexity for little gain.
- YAML-specific raw value snippets. Strict JSON remains the common structured
  value language unless a concrete source-preservation need appears.
