package countersign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/canonjson"
)

// EncodeRecord validates and canonically encodes one countersign record.
func EncodeRecord(record Record) ([]byte, error) {
	if err := validateRecord(record); err != nil {
		return nil, err
	}
	return canonjson.Marshal(record)
}

// DecodeRecord strictly decodes and validates one countersign record.
func DecodeRecord(reader io.Reader) (Record, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return Record{}, fmt.Errorf("countersign: read record: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var record Record
	if err := decoder.Decode(&record); err != nil {
		return Record{}, fmt.Errorf("countersign: decode record: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Record{}, fmt.Errorf("countersign: trailing data after record")
		}
		return Record{}, fmt.Errorf("countersign: trailing data after record: %w", err)
	}
	if err := validateRequiredJSON(raw); err != nil {
		return Record{}, err
	}
	if err := validateRecord(record); err != nil {
		return Record{}, err
	}
	canonical, err := canonjson.Marshal(record)
	if err != nil {
		return Record{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Record{}, fmt.Errorf("countersign: record is not byte-canonical")
	}
	return record, nil
}

func recordDigest(record Record) (string, error) {
	record.Digest = ""
	return canonjson.Digest(record)
}

func validateRequiredJSON(raw []byte) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return fmt.Errorf("countersign: inspect required fields: %w", err)
	}
	if err := requireFields("record", top, []string{"schema", "repository", "forge", "change_id", "candidate_sha", "obligation", "freshness", "approvals", "reduction", "verdict", "witnesses", "digest"}); err != nil {
		return err
	}
	obligation, err := objectField(top, "obligation")
	if err != nil {
		return err
	}
	if err := requireFields("obligation", obligation, []string{"transition", "scheme", "kind", "role", "required_count", "governance_profile_id", "governance_profile_digest", "separation_rule"}); err != nil {
		return err
	}
	freshness, err := objectField(top, "freshness")
	if err != nil {
		return err
	}
	if err := requireFields("freshness", freshness, []string{"policy_id", "policy_digest", "evaluated_at", "observed_at", "maximum_observation_age_seconds", "maximum_approval_age_seconds", "provider_snapshot_id"}); err != nil {
		return err
	}
	reduction, err := objectField(top, "reduction")
	if err != nil {
		return err
	}
	if err := requireFields("reduction", reduction, []string{"eligible_approval_ids", "distinct_principal_ids", "eligible_count", "required_count", "freshness_verdict", "separation_verdict"}); err != nil {
		return err
	}
	var approvals []map[string]json.RawMessage
	if err := json.Unmarshal(top["approvals"], &approvals); err != nil || approvals == nil {
		return fmt.Errorf("countersign: approvals must be a non-null array")
	}
	for i, approval := range approvals {
		prefix := fmt.Sprintf("approvals[%d]", i)
		if err := requireFields(prefix, approval, []string{"approval_id", "approval_ref", "state", "approved_at", "updated_at", "candidate_sha", "principal_resolution", "provider_witnesses"}); err != nil {
			return err
		}
		resolution, err := objectField(approval, "principal_resolution")
		if err != nil {
			return fmt.Errorf("countersign: %s: %w", prefix, err)
		}
		if err := requireFields(prefix+".principal_resolution", resolution, []string{"claim", "state", "witnesses"}); err != nil {
			return err
		}
		claim, err := objectField(resolution, "claim")
		if err != nil {
			return fmt.Errorf("countersign: %s: %w", prefix, err)
		}
		if err := requireFields(prefix+".principal_resolution.claim", claim, []string{"trust_source", "subject"}); err != nil {
			return err
		}
		var kernelWitnesses []map[string]json.RawMessage
		if err := json.Unmarshal(resolution["witnesses"], &kernelWitnesses); err != nil || kernelWitnesses == nil {
			return fmt.Errorf("countersign: %s.principal_resolution.witnesses must be a non-null array", prefix)
		}
		for j, witness := range kernelWitnesses {
			if err := requireFields(fmt.Sprintf("%s.principal_resolution.witnesses[%d]", prefix, j), witness, []string{"code", "source_id"}); err != nil {
				return err
			}
		}
		var providerWitnesses []map[string]json.RawMessage
		if err := json.Unmarshal(approval["provider_witnesses"], &providerWitnesses); err != nil || providerWitnesses == nil {
			return fmt.Errorf("countersign: %s.provider_witnesses must be a non-null array", prefix)
		}
		for j, witness := range providerWitnesses {
			if err := requireFields(fmt.Sprintf("%s.provider_witnesses[%d]", prefix, j), witness, []string{"name", "value"}); err != nil {
				return err
			}
		}
	}
	for _, field := range []string{"witnesses"} {
		var values []string
		if err := json.Unmarshal(top[field], &values); err != nil || values == nil {
			return fmt.Errorf("countersign: %s must be a non-null array", field)
		}
	}
	for _, field := range []string{"eligible_approval_ids", "distinct_principal_ids"} {
		var values []string
		if err := json.Unmarshal(reduction[field], &values); err != nil || values == nil {
			return fmt.Errorf("countersign: reduction.%s must be a non-null array", field)
		}
	}
	return nil
}

func requireFields(object string, fields map[string]json.RawMessage, required []string) error {
	for _, field := range required {
		value, ok := fields[field]
		if !ok {
			return fmt.Errorf("countersign: %s.%s is missing", object, field)
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("countersign: %s.%s must not be null", object, field)
		}
	}
	return nil
}

func objectField(parent map[string]json.RawMessage, field string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(parent[field], &object); err != nil || object == nil {
		return nil, fmt.Errorf("%s must be a non-null object", field)
	}
	return object, nil
}
