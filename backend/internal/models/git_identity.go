package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GitIdentity struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	Token     string    `json:"token"` // Check: Should this be json:"-" to prevent accidental leak? Yes, usually.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (GitIdentity) TableName() string {
	return "git_identities"
}

func (g *GitIdentity) BeforeCreate(tx *gorm.DB) (err error) {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	return
}
