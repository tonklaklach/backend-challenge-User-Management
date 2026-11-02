package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	HTTPAddress   string
	MongoURI      string
	MongoDatabase string
	JWTSecret     string
	JWTTTL        time.Duration
}

func Load() (Config, error) {
	c := Config{
		HTTPAddress:   env("HTTP_ADDRESS", ":8080"),
		MongoURI:      env("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase: env("MONGO_DATABASE", "backend_challenge"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
	}
	if len(c.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must contain at least 32 bytes")
	}
	ttl, err := time.ParseDuration(env("JWT_TTL", "1h"))
	if err != nil || ttl < time.Second {
		return Config{}, errors.New("JWT_TTL must be a duration of at least one second")
	}
	c.JWTTTL = ttl
	return c, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
