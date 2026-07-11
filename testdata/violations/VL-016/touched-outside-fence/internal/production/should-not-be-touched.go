// should-not-be-touched.go is a marker file representing a spike build
// branch's diff touching a path outside verdi.yaml's spike_paths: allowlist
// (02 §Lint rules VL-016, 01 §Store manifest "spike_paths"). VL-016 is not
// implemented until a later phase; this is the skeleton fixture V1-P1's
// appendix asks for — a second copy of the spike's build-branch diff
// touching a disallowed path, for VL-016's negative case once the rule
// lands and a real build-branch diff mechanism exists to check it against.
package production
