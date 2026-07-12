import { chromium } from "@playwright/test";
const out = "/Users/johnyang/.claude/jobs/f8ad4a26/tmp";
const prefix = process.argv[2] || "wip";
const targets = [
  ["authoring", "http://127.0.0.1:4173/board/spec/refi-decline-flow"],
  ["review", "http://127.0.0.1:4173/board/spec/stale-decline-notices"],
  ["readonly", "http://127.0.0.1:4173/board/spec/stale-decline"],
  ["empty", "http://127.0.0.1:4173/board/spec/income-verification"],
];
const browser = await chromium.launch();
for (const scheme of ["light", "dark"]) {
  const ctx = await browser.newContext({ colorScheme: scheme, viewport: { width: 1600, height: 1000 } });
  const page = await ctx.newPage();
  for (const [name, url] of targets) {
    await page.goto(url, { waitUntil: "networkidle" });
    await page.waitForTimeout(300);
    await page.screenshot({ path: `${out}/${prefix}-${name}-${scheme}.png` });
  }
  await ctx.close();
}
await browser.close();
console.log("done");
