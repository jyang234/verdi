# Vendored asset: mermaid.min.js

05 §Verdi-dex mechanics gives dex a client-side JavaScript budget of
exactly three items, the first of which is mermaid diagram rendering. The
spec asks that mermaid be vendored (rather than pulled from a CDN at
request time) so the published site is fully self-contained: no third-party
origin a reader's browser has to trust or reach, and no risk of a CDN going
away silently breaking every diagram on the site.

## Provenance

- **Source:** `https://cdn.jsdelivr.net/npm/mermaid@10.9.1/dist/mermaid.min.js`
  (jsDelivr's mirror of the `mermaid` npm package, itself built from
  `https://github.com/mermaid-js/mermaid`)
- **Version:** 10.9.1
- **Fetched:** 2026-07-11
- **SHA-256:** `61b335a46df05a7ce1c98378f60e5f3e77a7fb608a1056997e8a649304a936d6`
- **License:** MIT (Copyright (c) 2014 - 2022 Knut Sveidqvist), fetched
  alongside from `https://unpkg.com/mermaid@10.9.1/LICENSE` and reproduced
  below verbatim.

Re-vendoring: fetch the same URL pattern for a newer version, update the
version/date/sha256 lines above, and update the corresponding usage note in
`internal/dex/assets.go`. Nothing in `internal/dex` depends on mermaid's
internal API beyond the documented `mermaid.initialize({...})` UMD global
dex's page template calls after loading this file — an upgrade should be a
drop-in file replacement.

## License text

The MIT License (MIT)

Copyright (c) 2014 - 2022 Knut Sveidqvist

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
