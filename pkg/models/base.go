package models

import (
	"database/sql"

	"gorm.io/gorm"
)

// Model is a basic GoLang struct based on gorm.Model with the JSON tags for openapi3gen
type Model struct {
	ID        uint           `gorm:"primarykey" json:"ID,omitempty"`
	CreatedAt sql.NullTime   `json:"CreatedAt,omitempty"`
	UpdatedAt sql.NullTime   `json:"UpdatedAt,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"DeletedAt,omitempty"`
}
