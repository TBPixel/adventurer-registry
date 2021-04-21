package pq

import (
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/tbpixel/adventurer-registry/characters"
)

type sqlCharacter struct {
	AuthorID  string    `db:"author_id"`
	GuildID   string    `db:"guild_id"`
	Name      string    `db:"name"`
	Profile   string    `db:"profile"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type RegistryDB struct {
	db *sqlx.DB
}

// Characters returns a list of registered characters in a guild
func (r RegistryDB) Characters(guildID string) ([]characters.Character, error) {
	var chars []sqlCharacter
	err := r.db.Select(&chars, "SELECT * FROM characters WHERE guild_id = $1", guildID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []characters.Character{}, nil
		}

		return nil, err
	}

	return sqlToCharacters(chars), nil
}

// Find a character by their name and guild ID
func (r RegistryDB) Find(name, guildID string) (*characters.Character, error) {
	row := r.db.QueryRowx("SELECT * FROM characters WHERE name = $1 AND guild_id = $2 LIMIT 1", name, guildID)

	var char sqlCharacter
	err := row.StructScan(&char)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, characters.ErrNotFound
		}

		return nil, err
	}

	mapped := sqlToCharacters([]sqlCharacter{char})
	return &mapped[0], nil
}

// FindByAuthorID a character by their name and guild ID
func (r RegistryDB) FindByAuthorID(name, authorID string) (*characters.Character, error) {
	row := r.db.QueryRowx("SELECT * FROM characters WHERE name = $1 AND author_id = $2 LIMIT 1", name, authorID)

	var char sqlCharacter
	err := row.StructScan(&char)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, characters.ErrNotFound
		}

		return nil, err
	}

	mapped := sqlToCharacters([]sqlCharacter{char})
	return &mapped[0], nil
}

// Create a new character in the registry
func (r RegistryDB) Create(character characters.Character) (*characters.Character, error) {
	char, _ := r.Find(character.Name, character.GuildID)
	if char != nil {
		return nil, characters.ErrExists
	}

	stmt, err := r.db.Preparex(`
		INSERT INTO characters (author_id, guild_id, name, profile)
		VALUES ($1, $2, $3, $4)
	`)
	if err != nil {
		return nil, err
	}

	_, err = stmt.Exec(character.AuthorID, character.GuildID, character.Name, character.Profile)
	if err != nil {
		return nil, err
	}

	return r.Find(character.Name, character.GuildID)
}

// Update a character profile in the registry
func (r RegistryDB) Update(name, profile, guildID string) (*characters.Character, error) {
	char, _ := r.Find(name, guildID)
	if char == nil {
		return nil, characters.ErrNotFound
	}

	_, err := r.db.Exec(`
		UPDATE characters
		SET profile = $1, updated_at = NOW()
		WHERE name = $2 AND guild_id = $3
	`, profile, name, guildID)
	if err != nil {
		return nil, err
	}

	return r.Find(name, guildID)
}

// Delete a character permanently, found by name and guild_id
func (r RegistryDB) Delete(name, authorID, guildID string) error {
	// ensure delete is safe even if the character does not exist
	char, _ := r.Find(name, guildID)
	if char == nil {
		return nil
	}

	if char.AuthorID != authorID {
		return characters.ErrPermissionDenied
	}

	stmt, err := r.db.Preparex("DELETE FROM characters WHERE name = $1 AND guild_id = $2")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(name, guildID)
	return err
}

// DeleteByAuthorID a character permanently, found by name and author_id
func (r RegistryDB) DeleteByAuthorID(name, authorID string) error {
	// ensure delete is safe even if the character does not exist
	char, _ := r.FindByAuthorID(name, authorID)
	if char == nil {
		return nil
	}

	if char.AuthorID != authorID {
		return characters.ErrPermissionDenied
	}

	stmt, err := r.db.Preparex("DELETE FROM characters WHERE name = $1 AND author_id = $2")
	if err != nil {
		return err
	}

	_, err = stmt.Exec(name, authorID)
	return err
}

// CharactersByAuthor returns a list of registered by an author
func (r RegistryDB) CharactersByAuthor(authorId string) ([]characters.Character, error) {
	var chars []sqlCharacter
	err := r.db.Select(&chars, "SELECT * FROM characters WHERE author_id = $1", authorId)
	if err != nil {
		if err == sql.ErrNoRows {
			return []characters.Character{}, nil
		}

		return nil, err
	}

	return sqlToCharacters(chars), nil
}

func sqlToCharacters(chars []sqlCharacter) (c []characters.Character) {
	for _, character := range chars {
		c = append(c, characters.Character{
			AuthorID:  character.AuthorID,
			GuildID:   character.GuildID,
			Name:      character.Name,
			Profile:   character.Profile,
			CreatedAt: character.CreatedAt,
			UpdatedAt: character.UpdatedAt,
		})
	}

	return c
}
