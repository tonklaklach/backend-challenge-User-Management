package config

import "testing"

func TestLoad(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("JWT_TTL", "30m")
	t.Setenv("HTTP_ADDRESS", ":9090")
	t.Setenv("MONGO_URI", "mongodb://localhost:27017")
	t.Setenv("MONGO_DATABASE", "test")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.JWTTTL.Seconds() != 1800 || c.HTTPAddress != ":9090" || c.MongoDatabase != "test" {
		t.Fatalf("unexpected config: ttl=%v address=%s database=%s", c.JWTTTL, c.HTTPAddress, c.MongoDatabase)
	}
	for _, ttl := range []string{"invalid", "0s", "-1h"} {
		t.Setenv("JWT_TTL", ttl)
		if _, err := Load(); err == nil {
			t.Fatalf("accepted TTL %q", ttl)
		}
	}
	t.Setenv("JWT_TTL", "1h")
	t.Setenv("JWT_SECRET", "short")
	if _, err := Load(); err == nil {
		t.Fatal("accepted short secret")
	}
}
