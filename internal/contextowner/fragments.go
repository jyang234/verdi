package contextowner

// ValidateRedactedSegment checks the segment metadata and authenticated canonical
// JSON bytes. It validates a typed fragment without changing or normalizing it.
func ValidateRedactedSegment(segment RedactedSegment) error {
	_, err := validRedactedSegment(segment)
	return err
}

// SegmentReference derives the owner segment reference for a canonical SHA-256
// digest, refusing malformed digests.
func SegmentReference(digest string) (string, error) {
	return segmentReference(digest)
}

// ValidateSegmentReference checks the exact owner segment reference grammar.
func ValidateSegmentReference(reference string) error {
	return validateSegmentReference(reference)
}

// ValidateExecutionKey checks the required flight, lane, and epoch text. Enclosing
// artifacts and wire documents may impose additional constraints.
func ValidateExecutionKey(key ExecutionKey) error {
	return validateExecutionKey(key)
}

// ValidateFlightStateSnapshot checks the typed snapshot fragment and nested
// execution request. It does not establish relationships with other artifacts or
// validate revision history, and does not change or normalize the supplied value.
func ValidateFlightStateSnapshot(snapshot FlightStateSnapshot) error {
	_, err := validFlightStateSnapshot(snapshot)
	return err
}

// ValidateFlightStateSnapshotFields checks only snapshot key and scalar fields.
// It does not validate Request and is incomplete for wire admission. Domain
// callers must separately validate their typed execution request with its owner;
// public wire callers must use full snapshot or operation validation instead.
func ValidateFlightStateSnapshotFields(snapshot FlightStateSnapshot) error {
	return validateFlightStateSnapshotFields(snapshot)
}
