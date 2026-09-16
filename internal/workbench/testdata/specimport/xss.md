# Escaped Import

## Problem

Operators paste <script>window.__xss = 1</script> markup and <img src=x onerror="window.__xss = 2"> tags by mistake.

## Outcome

Every source byte renders as text: `<b>never</b>` as markup.

## Acceptance Criteria

- The preview shows <em>literal</em> angle brackets.
- Nothing in the source runs in the browser.
