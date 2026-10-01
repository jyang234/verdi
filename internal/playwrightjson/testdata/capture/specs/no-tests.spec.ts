// Capture fixture (not a suite test): a spec file that exists but declares no
// tests (every test removed, or none written yet). Playwright stops a run of
// it alone with "No tests found" unless --pass-with-no-tests is given; with
// it, the file's named title paths are simply absent (SI-308).
export {};
