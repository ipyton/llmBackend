package entities

import "time"

type Session struct {
	SessionId    string
	LastModified time.Time
	Role         string
	Title        string
}
