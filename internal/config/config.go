package config

import "os"

type Config struct {
	DBHost string
	DBPort string
	DBUser string
	DBPass string
	DBName string
	DBSSL  string
	Port   string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() Config {
	return Config{
		DBHost: env("DB_HOST", "localhost"),
		DBPort: env("DB_PORT", "5432"),
		DBUser: env("DB_USER", "postgres"),
		DBPass: env("DB_PASSWORD", "postgres"),
		DBName: env("DB_NAME", "relief"),
		DBSSL:  env("DB_SSLMODE", "disable"),
		Port:   env("PORT", "8080"),
	}
}
