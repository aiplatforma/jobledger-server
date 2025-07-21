package models

import "time"

type Token struct {
	ID        int           `db:"id" json:"id"`
	Comment   string        `db:"comment" json:"comment"`
	Duration  time.Duration `db:"duration" json:"duration"`
	Token     string        `db:"token" json:"token"`
	CreatedAt time.Time     `db:"created_at" json:"created_time"`
}
