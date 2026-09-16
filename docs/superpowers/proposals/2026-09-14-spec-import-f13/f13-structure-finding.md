# F13 source-structure finding: explicit statements absent

Status: owner-identified finding, 2026-09-14; source correction not performed.

**Source:** Verdi-ATC at `c346c005d117dfb090e1dedd5a89f45080875a5e`,
`docs/superpowers/plans/2026-08-24-verdi-atc-stage-1-orchestration.md`,
selected F13 section lines 602–671. Exact bytes: `sources/primary-f13.md`.

**Observed structure:** Task title, Files, Interfaces and four implementation
steps. There is no explicitly labeled Problem or Outcome statement in this
selected feature definition. This finding is scoped to the selected source;
it does not assert that no related discussion exists elsewhere in ATC.

**Consequence:** The importer cannot extract a labeled pair that this source
does not declare. It must show the missing labels, retaining all source text.
It must not substitute an inferred summary or a supporting slice's Goal.

**Importer requirement:** A properly labeled Problem/Outcome pair in native
Verdi format or the supported Markdown structure must populate the corresponding
fields directly, preserving wording, multiline content and source spans.
This positive case is a required acceptance test, separate from F13's negative
case. Missing/empty fields and ambiguous duplicate or cross-target labels must
be reported explicitly. A later approved source correction would be imported
from its own new pinned revision; the original snapshot is never rewritten.

**Disposition:** Carry as an upstream structure finding for F13. Correcting
F13's governing definition remains a distinct source authoring/review task.
The prototype and importer scope do not silently make that authority change.
