package entities

import (
	"github.com/gocql/gocql"
	"time"
)

type Response struct {
	Data         string
	SendTime     time.Time
	CompleteTime time.Time
	SessionID    gocql.UUID
}
