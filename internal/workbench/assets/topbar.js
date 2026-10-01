// The workbench top bar's script (spec/chrome-and-tokens-v2): every
// workbench page's one bar, served at /assets/topbar.js so the shared
// layout and the four page shells all reference the same asset. New
// behavior ships in a new asset of at most 64 KiB (spec/workbench-redesign
// co-1; TestTopBarAsset_ServedWithinBudget) rather than growing
// boardspec.js. The bar is complete before this script runs; the script
// only enhances it.
