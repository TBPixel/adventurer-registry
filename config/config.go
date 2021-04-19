package config

type Config struct {
	Database DB
	Discord  Discord
}

type Discord struct {
	Token string
}

type DB struct {
	URL string
}
