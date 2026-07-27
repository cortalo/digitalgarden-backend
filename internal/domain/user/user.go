package user

import "errors"

// ErrNotFound is returned by the service/repository layer when no row
// matches.
var ErrNotFound = errors.New("user: not found")

// User is the business object (BO): the core entity, with no knowledge of
// HTTP or the database.
type User struct {
	ID        int64
	GoogleSub string
	Name      string
	Email     string
}
