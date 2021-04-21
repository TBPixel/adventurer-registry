package pq

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/tbpixel/adventurer-registry/config"
)

const schema = `
CREATE TABLE IF NOT EXISTS characters (
	author_id varchar(255) NOT NULL,
    guild_id varchar(255) NOT NULL,
    name varchar(255) NOT NULL,
    profile text NOT NULL,
    created_at TIMESTAMP NOT NULL Default NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_characters_guild_id_name ON characters(guild_id, name);
CREATE UNIQUE INDEX IF NOT EXISTS ids_characters_author_id ON characters(author_id);
`

// Connect to the remote database, running a ping to verify
func Connect(conf config.DB) (*DB, error) {
	conn, err := sqlx.Connect("postgres", conf.URL)
	if err != nil {
		return nil, err
	}

	err = initSchema(conn)
	if err != nil {
		return nil, err
	}

	return &DB{
		conn:     conn,
		Registry: RegistryDB{db: conn},
	}, nil
}

// DB wraps our database layer
type DB struct {
	conn     *sqlx.DB
	Registry RegistryDB
}

// Disconnect from the database, defer for a graceful application shutdown
func (db *DB) Disconnect() error {
	return db.conn.Close()
}

func initSchema(conn *sqlx.DB) error {
	_, err := conn.Exec(schema)

	return err
}
