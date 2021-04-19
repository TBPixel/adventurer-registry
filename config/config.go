package config

type Config struct {
	Port     string
	Database DB
	Discord  Discord
}

type Discord struct {
	Token string
}

type DB struct {
	URL string
}
