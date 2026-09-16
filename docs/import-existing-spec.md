# Import an existing specification

Use import when your feature or story already exists in Markdown. Verdi copies the selected content into a new proposed specification and opens its ordinary board. The canonical specification remains the board's source; the importer does not create a second card model or call AI.

On this page: [Browser import](#in-the-browser) · [Review imported content](#review-imported-content) · [F13 bundle](#import-the-f13-reference-bundle) · [Supported inputs](#supported-inputs) · [Markdown example](#labeled-markdown-example) · [CLI and source records](#cli-and-source-records)

## In the browser

Start Verdi in a clean configured project, open its home page, and choose **Import existing spec**. Select your source files and any line ranges, then choose a new specification name and its class. Review the imported fields, supply missing content, select the evidence each acceptance criterion requires, and acknowledge the source material retained outside the canonical specification.

Explicit Problem and Outcome headings prepopulate those fields. If they are absent, the importer reports them as missing. You may author them or explicitly defer both using the disclosed TODO representation; deferral does not establish readiness for review. F13 is the reference case for missing labels, not permission to infer statements from unlabeled prose.

Select **Preview** to check the current choices. Any edit requires another preview before confirmation. When it is ready, check the confirmation and select **Create proposal**, then follow its board link. Make a supported edit and reload to check it persisted. The source-record link describes the original import separately; after a later edit it can correctly say that today's specification differs from the imported revision.

If the connection fails during creation, use the visible retry control while the
inputs are unchanged. It retries the same confirmed request and can return the
already-created proposal. If you edit the inputs, that retry is invalidated; the
page keeps the earlier outcome disclosed. A late success identifies the proposal
created from the earlier inputs. Inspect the named board and source record before
trying to create another proposal with the same name.

Human browser import does not require an AI assistance policy. Existing tracker, requiredness, approval and evidence rules still apply. A feature needs no issue tracker; an imported story must meet the existing story requirements.

## Review imported content

Preview prepares the fields without creating a proposal. Review the statements
and acceptance criteria, then resolve the items shown beside them. The page's
advanced controls let you map exact source ranges and add relationships when
needed; ordinary evidence choices do not require manual mapping.

Evidence choices declare what must be produced later. They do not upload tests,
create attestations or mark a criterion proven. Existing validation requires
attestation for every feature criterion. Choose any additional kinds required
for your change, using the explanations beside the controls. An explicit
apply-to-all action can apply the same choices to the displayed criteria.

If the source lacks Problem or Outcome, write the missing statement explicitly,
or choose to leave both as TODOs. That choice replaces both statements, including
any that were copied; the placeholders remain visibly incomplete. Choose whether
to keep the remaining source text as reference material stored with the import.
Retaining text does not turn it into specification fields or resolve missing
requirements.

Changing an input makes the previous preview out of date. The displayed field
choices are your current inputs; earlier findings describe the last check.
Preview again to obtain current findings. Confirm and create only after that
fresh preview is ready.

## Import the F13 reference bundle

The `f13-reference-v1` profile is a fixed mapping for the Verdi-ATC prototype at
commit `c346c005d117dfb090e1dedd5a89f45080875a5e`. It supports that exact
selection, not arbitrary implementation plans. In a clean configured copy of
that project, choose these files under `docs/superpowers/plans/`:

| File | Selection |
| --- | --- |
| `2026-08-24-verdi-atc-stage-1-orchestration.md` | Primary, lines 602–671 inclusive |
| `2026-09-12-f13-review-validation.md` | Whole supporting file |
| `2026-09-12-f13-transition-core.md` | Whole supporting file |
| `2026-09-12-f13-review-journal.md` | Whole supporting file |

Choose the F13 reference format, class `feature`, a new name such as
`f13-gatekeeper`, and a title. Preview shows eight acceptance criteria. The
primary lacks labeled Problem and Outcome statements: write them explicitly or
choose the disclosed pair of TODOs. Review the criteria and choose their expected
evidence, including the existing feature requirement for attestation. Choose **Keep the remaining source text as reference material** to preserve the
supporting plans and the unmapped primary text. This bundle cannot be created
while that text has no disposition; retaining it does not turn it into additional
board fields. Preview again after changes,
then confirm and create when ready.

Changed primary bytes are refused by this profile. Use a properly labeled
Markdown specification or explicit manual mapping for different content; do not
assume changing the filename makes it the same reference. Import creates a new
proposal, so a pre-existing `f13-gatekeeper` target is a collision to inspect,
not a branch to overwrite.

## Supported inputs

- Native Verdi specifications preserve exact eligible source bytes and require matching target identity. Choose `native` and enter the same name, class and title as the source; change the source and reupload it if you intend to change that metadata.
- The labeled Markdown profile recognizes the documented Problem, Outcome and flat Acceptance Criteria structure. Ambiguities require explicit correction.
- The `manual-v1` profile lets you select source spans or supply explicitly authored text. User changes remain distinguishable from copied source. Mapping offsets count UTF-8 bytes within the selected slice, from the inclusive start to the exclusive end; line selection and mapping offsets are different coordinates.
- The F13 reference profile accepts the pinned prototype content. It is not a general inference mode for arbitrary stage plans.

A bundle is an explicit set of selected files. The importer does not traverse directories, fetch URLs, open archives or execute instructions in source text. Select 1–32 nonempty UTF-8 files. Limits are 2 MiB per file, 8 MiB of supplied source bytes and a 12 MiB JSON request envelope. Nonempty custom template slots that cannot be preserved are refused; inspect the diagnostic instead of assuming every template can be rewritten.

## Labeled Markdown example

```markdown
# Request receipts

## Problem
People cannot tell whether a submitted request was saved.

## Outcome
Every saved request has an identifier that can be looked up.

## Acceptance Criteria
- Saving a request returns a stable identifier.
- Looking up an unknown identifier reports that no request was found.
```

The title is the first real heading. Field headings are exactly one level deeper.
Supported labels are Problem (or Problem Statement), Outcome (or Outcome
Statement), Acceptance Criteria, Constraints, Decisions and Open Questions;
matching ignores case and surrounding spaces. ATX and setext headings work.
Object sections use flat bullet lists. Duplicate sections, nested lists or
multiple top-level targets require correction rather than a guessed mapping.
Leading YAML metadata and text before the title are retained without becoming
field authority. Code-fence headings are not treated as fields.

Evidence requirements are selected separately and are never inferred from the
criterion's wording. If the source already uses IDs such as `ac-1:`, resolve the
ID explicitly rather than silently replacing it. Supporting sources can be kept as reference material when you explicitly choose
to retain the remaining source text, or used by an explicit mapping.

Line selections are inclusive and start at 1; both endpoints are required.
Limits apply to the supplied file before selection. Source reading rejects
symlink components, nonregular files, absolute file paths and traversal. A source
label is displayed as metadata and is never a destination path.

## CLI and source records

The CLI exposes `source`, `preview`, `apply` and `record` under
`verdi design import`. The following read-only example requires Python 3. Save
the Markdown example above as `docs/request-receipts.md` in your configured
project and commit your intended project changes first. Import preview requires
a clean project. Keep generated request files outside it:

```sh
VERDI_IMPORT_DIR="$(mktemp -d)"
verdi design import source --root "$PWD" --file docs/request-receipts.md \
  > "$VERDI_IMPORT_DIR/source.json"
python3 - "$VERDI_IMPORT_DIR" <<'PYTHON'
import json
import sys
from pathlib import Path

folder = Path(sys.argv[1])
source = json.loads((folder / "source.json").read_text())
request = {
    "schema": "verdi.spec-import-request/v1",
    "format": "markdown-v1",
    "target": {
        "slug": "request-receipts",
        "class": "feature",
        "title": "Request receipts",
    },
    "primary": source["id"],
    "sources": [source],
    "mappings": [
        {"target": "ac-1", "evidence": ["static", "attestation"]},
        {"target": "ac-2", "evidence": ["static", "attestation"]},
    ],
    "defer_statements": False,
    "retain_unmapped": True,
}
(folder / "request.json").write_text(json.dumps(request) + "\n")
PYTHON
verdi design import preview --request "$VERDI_IMPORT_DIR/request.json" \
  > "$VERDI_IMPORT_DIR/preview.json"
python3 -m json.tool "$VERDI_IMPORT_DIR/preview.json"
```

This example explicitly requires static evidence and attestation for both
criteria. Choose requirements appropriate to your actual change and project
model. Selecting a kind does not provide the evidence. `retain_unmapped: true`
acknowledges that source content outside mapped fields is retained in the import
record; review its coverage before confirming creation. For a bundle, call
`source` for each explicitly selected file and include the resulting objects in
`sources`, with unique IDs and one `primary` ID. `source` embeds the supplied
bytes as base64; do not paste Markdown into that JSON property.

After inspecting a ready preview, an authorized delegated harness can apply the
same request with its returned `digest`:

```text
verdi design import apply --request <request.json> --preview <digest> --harness <harness-id> [--session <session-id>]
verdi design import record --branch design/request-receipts --spec request-receipts
```

`apply` creates the design branch without switching your checkout. Use the
returned `board_path` on your running Verdi server to open that branch's board.
`record` requires a created import and reads its committed proof. Request input
can be a file or `--request -` for stdin; use the exact same bytes and options
when confirming a preview.

CLI apply uses a delegated-agent actor and requires a harness identifier and applicable project policy. It has no human bypass. For human adoption without that policy, use the browser. Read-only preview needs no assistance policy.

A preview with unresolved findings is structured output with exit 1; successful commands exit 0; malformed requests and operational failures exit 2. Inspect the findings, not just the exit code. Retry an uncertain successful creation with the same request, preview digest, actor and binary. A changed branch or incompatible proof requires source-record inspection; do not reset the branch or create a duplicate to conceal the refusal.

Source records verify the stored selected content and original import revision. A full-file fingerprint is not proof that unselected bytes were retained. Byte coverage is not semantic completeness, third-party authorship, approval, acceptance or CI proof. Expected evidence declarations say what must be produced; importing them does not produce that evidence.
