// FIXME: golangci-lint
// nolint:govet,revive
package models

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"

	"gorm.io/gorm"
)

// Model is a basic GoLang struct based on gorm.Model with the JSON tags for openapi3gen
type Model struct {
	ID        uint           `gorm:"primarykey" json:"ID,omitempty"`
	CreatedAt EdgeAPITime    `gorm:"index" json:"CreatedAt,omitempty"`
	UpdatedAt EdgeAPITime    `gorm:"index" json:"UpdatedAt,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"DeletedAt,omitempty"`
}

// ModelWithoutTimestamps is a basic GoLang struct based on gorm.Model without timestamps
type ModelWithoutTimestamps struct {
	ID uint `gorm:"primarykey" json:"ID,omitempty"`
}

// EdgeAPITime is a time.Time with a valid flag.
type EdgeAPITime sql.NullTime

// Scan implements the Scanner interface.
func (t *EdgeAPITime) Scan(value interface{}) error {
	return (*sql.NullTime)(t).Scan(value)
}

// Value implements the driver Valuer interface.
func (t EdgeAPITime) Value() (driver.Value, error) {
	if !t.Valid {
		return nil, nil
	}
	return t.Time, nil
}

// MarshalJSON implements the json.Marshaler interface for EdgeAPITime.
func (t EdgeAPITime) MarshalJSON() ([]byte, error) {
	if t.Valid {
		return json.Marshal(t.Time)
	}
	return json.Marshal(nil)
}

// UnmarshalJSON implements the json.Unmarshaler interface for EdgeAPITime.
func (t *EdgeAPITime) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		t.Valid = false
		return nil
	}
	err := json.Unmarshal(b, &t.Time)
	if err == nil {
		t.Valid = true
	}
	return err
}
