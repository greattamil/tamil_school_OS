// Package audit writes append-only audit entries inside the same transaction as the
// change they record. Per PRD 6.5, only changes are logged (not routine writes),
// and entries store deltas (before/after of the changed keys only), not full-row
// snapshots.
package audit

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Entry struct {
	ActorUserID uuid.UUID
	Action      string
	EntityType  string
	EntityID    uuid.UUID
	Before      map[string]any
	After       map[string]any
	Reason      string
	IPAddress   string
	DeviceID    string
}

// Write inserts an audit entry using tx, so the entry commits or rolls back with the
// change it describes. Call this from within db.WithTenantTx.
func Write(ctx context.Context, tx pgx.Tx, schoolID uuid.UUID, e Entry) error {
	var beforeJSON, afterJSON []byte
	var err error
	if e.Before != nil {
		if beforeJSON, err = json.Marshal(e.Before); err != nil {
			return err
		}
	}
	if e.After != nil {
		if afterJSON, err = json.Marshal(e.After); err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log (school_id, actor_user_id, action, entity_type, entity_id,
			before_values, after_values, reason, ip_address, device_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, '')::inet, NULLIF($10, ''))
	`, schoolID, e.ActorUserID, e.Action, e.EntityType, e.EntityID,
		nullableJSON(beforeJSON), nullableJSON(afterJSON), e.Reason, e.IPAddress, e.DeviceID)
	return err
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
