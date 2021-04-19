package characters

import (
	"fmt"
	"time"
)

var (
	ErrExists   = fmt.Errorf("character by that name already exists")
	ErrNotFound = fmt.Errorf("character by that name could not be found")
)

type Character struct {
	AuthorID  string
	GuildID   string
	Name      string
	Profile   string
	CreatedAt time.Time
	UpdatedAt time.Time
}
