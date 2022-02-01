package models

import (
	"time"

	"gorm.io/gorm"
)

// Model is a basic GoLang struct based on gorm.Model with the JSON tags for openapi3gen
type Model struct {
	ID        uint            `gorm:"primarykey" json:"ID,omitempty"`
	CreatedAt *time.Time      `json:"CreatedAt,omitempty"`
	UpdatedAt *time.Time      `json:"UpdatedAt,omitempty"`
	DeletedAt *gorm.DeletedAt `gorm:"index" json:"DeletedAt,omitempty"`
}
