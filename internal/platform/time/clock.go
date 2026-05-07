package time

import (
	"time"
)

type Clock interface {
	Now() time.Time
}

type clock struct{}

func (c *clock) Now() time.Time {
	return time.Now().UTC()
}

func NewClock() Clock {
	return &clock{}
}
