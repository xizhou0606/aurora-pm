package bitable

import (
	"math"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxSafeRevision uint64 = 9007199254740991

var collaborationIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func ValidateCellContract(contract CellContract) error {
	if contract.BaseRevision == 0 || contract.BaseRevision > MaxSafeRevision || ValidateCellLeaseProof(CellLeaseProof{
		RecordID: contract.RecordID, FieldID: contract.FieldID, LeaseID: contract.LeaseID,
		FencingToken: contract.FencingToken, ConnectionID: contract.ConnectionID,
		EditorInstanceID: contract.EditorInstanceID,
	}) != nil {
		return ErrInvalidInput
	}
	return nil
}

func ValidateCellLeaseProof(proof CellLeaseProof) error {
	leaseID, err := uuid.Parse(proof.LeaseID)
	if !validID(proof.RecordID) || !validID(proof.FieldID) || err != nil || leaseID == uuid.Nil ||
		leaseID.String() != proof.LeaseID || proof.FencingToken == 0 || proof.FencingToken > MaxSafeRevision ||
		!collaborationIdentifierPattern.MatchString(proof.ConnectionID) ||
		!collaborationIdentifierPattern.MatchString(proof.EditorInstanceID) {
		return ErrInvalidInput
	}
	return nil
}

func validID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func ValidateQuery(q Query) error {
	if q.LocateRecordID != "" && !validID(q.LocateRecordID) {
		return ErrInvalidInput
	}
	if !validID(q.ViewID) || q.Offset < 0 || q.Offset > MaxExportRows || q.Limit < 1 || q.Limit > MaxPageRows || len(q.Filters) > 20 || len(q.Sorts) > 8 {
		return ErrInvalidInput
	}
	for _, f := range q.Filters {
		if !validID(f.FieldID) {
			return ErrInvalidInput
		}
		switch f.Operator {
		case "eq", "neq", "contains", "gt", "lt":
		case "empty", "not_empty":
			if f.Value != nil {
				return ErrInvalidInput
			}
		default:
			return ErrInvalidInput
		}
	}
	seen := map[string]bool{}
	for _, s := range q.Sorts {
		if !validID(s.FieldID) || seen[s.FieldID] || s.Direction != "asc" && s.Direction != "desc" {
			return ErrInvalidInput
		}
		seen[s.FieldID] = true
	}
	return nil
}

func ValidateSearch(q Search) error {
	if err := ValidateQuery(q.Query); err != nil {
		return err
	}
	if strings.TrimSpace(q.Text) == "" || !utf8.ValidString(q.Text) || utf8.RuneCountInString(q.Text) > 200 {
		return ErrInvalidInput
	}
	return nil
}

func ValidateBatch(batch CommandBatch) error {
	id, err := uuid.Parse(batch.OperationID)
	if err != nil || id == uuid.Nil || id.String() != batch.OperationID || !batch.Atomic || batch.ExpectedSchemaRevision > MaxSafeRevision || len(batch.Commands) == 0 || len(batch.Commands) > MaxCommandRows {
		return ErrInvalidInput
	}
	seen, cells := map[string]bool{}, 0
	for _, c := range batch.Commands {
		if p := c.Position; p != nil {
			if (c.Kind != "create" && c.Kind != "move") || !ValidOperationID(p.RecordID) || p.Direction != "before" && p.Direction != "after" || p.ExpectedRevision == nil || *p.ExpectedRevision > MaxSafeRevision {
				return ErrInvalidInput
			}
		}
		switch c.Kind {
		case "create":
			if c.RecordID != "" || c.ExpectedRevision != nil || len(c.CellContracts) != 0 || len(c.CellLeaseProofs) != 0 {
				return ErrInvalidInput
			}
		case "update", "delete", "restore", "move":
			if !validID(c.RecordID) || seen[c.RecordID] {
				return ErrInvalidInput
			}
			if c.Kind == "update" {
				if c.ExpectedRevision != nil {
					return ErrInvalidInput
				}
			} else if c.ExpectedRevision == nil || *c.ExpectedRevision > MaxSafeRevision {
				return ErrInvalidInput
			}
			seen[c.RecordID] = true
			if c.Kind == "move" && (c.Position == nil || c.Position.RecordID == c.RecordID || len(c.Values) > 0) {
				return ErrInvalidInput
			}
			if (c.Kind == "delete" || c.Kind == "restore") && len(c.Values) > 0 || c.Kind == "update" && len(c.Values) == 0 {
				return ErrInvalidInput
			}
			if c.Kind == "update" {
				if len(c.CellContracts) != len(c.Values) || len(c.CellLeaseProofs) != 0 {
					return ErrInvalidInput
				}
				contractFields := map[string]bool{}
				for _, contract := range c.CellContracts {
					if ValidateCellContract(contract) != nil || contract.RecordID != c.RecordID ||
						contractFields[contract.FieldID] {
						return ErrInvalidInput
					}
					if _, ok := c.Values[contract.FieldID]; !ok {
						return ErrInvalidInput
					}
					contractFields[contract.FieldID] = true
				}
			} else if len(c.CellContracts) != 0 {
				return ErrInvalidInput
			} else if c.Kind == "move" || c.Kind == "delete" || c.Kind == "restore" {
				proofFields := map[string]bool{}
				for _, proof := range c.CellLeaseProofs {
					if ValidateCellLeaseProof(proof) != nil || proof.RecordID != c.RecordID || proofFields[proof.FieldID] {
						return ErrInvalidInput
					}
					proofFields[proof.FieldID] = true
				}
			}
		default:
			return ErrInvalidInput
		}
		cells += len(c.Values)
		for field, value := range c.Values {
			if !validID(field) {
				return ErrInvalidInput
			}
			if n, ok := value.(float64); ok && (math.IsNaN(n) || math.IsInf(n, 0) || math.Abs(n) > float64(MaxSafeRevision)) {
				return ErrInvalidInput
			}
			if text, ok := value.(string); ok && (!utf8.ValidString(text) || len(text) > 32768) {
				return ErrBudgetExceeded
			}
		}
	}
	if cells > MaxCommandCells {
		return ErrBudgetExceeded
	}
	return nil
}
