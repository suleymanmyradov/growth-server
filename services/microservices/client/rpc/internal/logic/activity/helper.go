package activitylogic

import (
	"time"
)

func ToUnix(t time.Time) int64 {
	return t.Unix()
}

func ToInt32(n int64) int32 {
	return int32(n)
}
