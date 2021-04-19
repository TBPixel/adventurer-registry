package config

import "fmt"

type Config struct {
	Database DB
	Discord  Discord
}

type Discord struct {
	Token string
}

type DB struct {
	Host         string
	User         string
	Password     string
	DatabaseName string
	SSLMode      bool
}

func (db DB) String() string {
	sslMode := "disable"
	if db.SSLMode {
		sslMode = "enable"
	}

	return fmt.Sprintf("user=%s password=%s dbname=%s host=%s sslmode=%s", db.User, db.Password, db.DatabaseName, db.Host, sslMode)
}
