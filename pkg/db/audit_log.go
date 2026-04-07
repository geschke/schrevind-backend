package db

import (
	"fmt"
	"time"
)

// Action constants for audit_log entries.
const (
	ActionCreate     = "create"
	ActionUpdate     = "update"
	ActionDelete     = "delete"
	ActionDeactivate = "deactivate"
	ActionGrant      = "grant"
	ActionRevoke     = "revoke"
	ActionTransfer   = "transfer"
)

type AuditLog struct {
	ID         int64  `json:"ID"`
	UserID     int64  `json:"UserID"`
	Action     string `json:"Action"`
	EntityType string `json:"EntityType"`
	EntityID   int64  `json:"EntityID"`
	Detail     string `json:"Detail,omitempty"`
	CreatedAt  int64  `json:"CreatedAt,omitempty"`
}

// WriteAuditLog appends a new entry to the audit log.
func (d *DB) WriteAuditLog(entry *AuditLog) error {
	if d == nil || d.SQL == nil {
		return fmt.Errorf("db not initialized")
	}
	if entry == nil {
		return fmt.Errorf("audit log entry is nil")
	}

	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}

	res, err := d.SQL.Exec(`
INSERT INTO audit_log (user_id, action, entity_type, entity_id, detail, created_at)
VALUES (?, ?, ?, ?, ?, ?);
`, entry.UserID, entry.Action, entry.EntityType, entry.EntityID, entry.Detail, entry.CreatedAt)
	if err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("write audit log last_insert_id: %w", err)
	}

	entry.ID = id
	return nil
}

// ListAuditLogByEntity returns all audit log entries for a given entity.
func (d *DB) ListAuditLogByEntity(entityType string, entityID int64) ([]AuditLog, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if entityID <= 0 {
		return nil, fmt.Errorf("entity_id must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT id, user_id, action, entity_type, entity_id, detail, created_at
  FROM audit_log
 WHERE entity_type = ?
   AND entity_id   = ?
 ORDER BY id ASC;
`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list audit log by entity: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]AuditLog, 0)
	for rows.Next() {
		var e AuditLog
		if err := rows.Scan(
			&e.ID,
			&e.UserID,
			&e.Action,
			&e.EntityType,
			&e.EntityID,
			&e.Detail,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit log entry: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit log by entity: %w", err)
	}

	return out, nil
}

// ListAuditLogByUser returns all audit log entries for a given user.
func (d *DB) ListAuditLogByUser(userID int64) ([]AuditLog, error) {
	if d == nil || d.SQL == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	if userID <= 0 {
		return nil, fmt.Errorf("user_id must be > 0")
	}

	rows, err := d.SQL.Query(`
SELECT id, user_id, action, entity_type, entity_id, detail, created_at
  FROM audit_log
 WHERE user_id = ?
 ORDER BY id ASC;
`, userID)
	if err != nil {
		return nil, fmt.Errorf("list audit log by user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make([]AuditLog, 0)
	for rows.Next() {
		var e AuditLog
		if err := rows.Scan(
			&e.ID,
			&e.UserID,
			&e.Action,
			&e.EntityType,
			&e.EntityID,
			&e.Detail,
			&e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit log entry: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit log by user: %w", err)
	}

	return out, nil
}
