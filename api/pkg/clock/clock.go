// Package clock implements usecases.Clock.
package clock

import "time"

type System struct{}

// Now returns UTC time cut to milliseconds, the precision MongoDB stores,
// so a value reads back from the database exactly as it was written.
func (System) Now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}
