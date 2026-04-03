package store

import "time"

func sqliteTimePtr(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.UTC().Format(sqliteTimeFormat)
}
