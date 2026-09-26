// Package ids implements usecases.IDGenerator.
package ids

import "github.com/google/uuid"

// UUIDv7 IDs start with a millisecond timestamp and increase within one process,
// so sorting them as strings sorts them by creation time. MessageRepository pages on this.
type UUIDv7 struct{}

func (UUIDv7) NewID() string {
	return uuid.Must(uuid.NewV7()).String()
}
