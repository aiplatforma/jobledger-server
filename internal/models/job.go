package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type MetadataMap map[string]interface{}

func (m *MetadataMap) Scan(src interface{}) error {
	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal MetadataMap value: %v", src)
	}
	return json.Unmarshal(bytes, m)
}

func (m MetadataMap) Value() (driver.Value, error) {
	return json.Marshal(m)
}

type Job struct {
	ID          int         `db:"id" json:"id"`
	Name        string      `db:"name" json:"name"`
	Type        string      `db:"type" json:"type"`
	State       string      `db:"state" json:"state"`
	CreatedAt   time.Time   `db:"created_at" json:"created_time"`
	StartedAt   *time.Time  `db:"started_at" json:"started_time"`
	CompletedAt *time.Time  `db:"completed_at" json:"completed_time"`
	Metadata    MetadataMap `db:"metadata" json:"metadata"`
}
