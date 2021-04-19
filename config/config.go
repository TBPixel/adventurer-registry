package config

type Config struct {
	Host     string
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
