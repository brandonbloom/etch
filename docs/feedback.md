Remaining feedback:


### ROUGH-10: Script quoting rules undocumented

`etch help scripts` doesn't explain how to quote values with spaces or JSON
objects. Unquoted `{"name":"first"}` in a script fails as "invalid JSON value".
Need single quotes: `'{"name":"first"}'`. A few examples would help.

---

### ROUGH-11: `--body` and `--task` can be combined silently

When both `--body` and `--task` are passed to `etch set` on a Markdown file,
`--task` silently wins and `--body` is ignored. Should error on conflicting
flags.

---

### ROUGH-12: `task add` / `list add` without `--section` appends to unpredictable location

Without `--section`, items are appended to the last section containing list
items. No warning. Could surprise users in files with multiple sections.

---

## Wish List

### WISH-1: `etch get` / `etch read` command for querying values

No read-side command exists. `etch get state.json status` to print the current
value would be useful for scripting.

---

### WISH-2: Negative array indexes (`$.items[-1]`)

Currently rejected: "negative indexes are not supported". Supporting at least
`-1` for "last element" would cover many use cases.

---

### WISH-3: JSONC/JSON5 support (trailing commas, comments)

Files like `tsconfig.json`, VS Code settings, and `.jsonc` files are common in
practice. Currently: parse error on trailing commas or `//` comments.

---

### WISH-4: `--pretty` / `--indent` flag for JSON creation

Fresh JSON files use compact single-line format. An option (or
`.editorconfig` detection) for human-readable formatting would be welcome.

---

### WISH-5: `section replace` option to preserve surrounding blank lines

A `--preserve-spacing` flag or automatic matching of the original section's
blank-line pattern would reduce diff noise.

---

### WISH-6: `.md` create template with frontmatter

`etch create newfile.md` produces an empty file. A minimal
`---\ntitle: newfile\n---\n` template would be more practical since most
Markdown workflows expect frontmatter.

---

### WISH-7: Batch script quoting examples in `etch help scripts`

Even 2-3 examples showing string quoting, JSON quoting, and multi-word values
in scripts would prevent a lot of trial and error.
