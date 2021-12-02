package models

import (
	"time"

	"gorm.io/gorm"
)

// Model is a basic GoLang struct based on gorm.Model with the JSON tags for openapi3gen
type Model struct {
	ID        uint           `gorm:"primarykey" json:"id,omitempty"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
	UpdatedAt time.Time      `json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
