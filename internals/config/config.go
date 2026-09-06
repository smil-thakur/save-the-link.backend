package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Mongo_db_username          string
	Mongo_db_password          string
	Mongo_db_connection_string string
	JWT_secret                 string
	AllowedOrigins             []string
}

func GetConfig() *Config {
	// .env is for local development only — on Render (and most hosts) env vars are
	// injected directly into the process, so no .env file exists. Don't fatal here.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found — reading configuration from the environment instead")
	}

	return &Config{
		Mongo_db_username:          os.Getenv("mongo_db_username"),
		Mongo_db_password:          os.Getenv("mongo_db_password"),
		Mongo_db_connection_string: os.Getenv("mongo_connection_string"),
		JWT_secret:                 os.Getenv("jwt_secret"),
		AllowedOrigins:             parseOrigins(os.Getenv("allowed_origins")),
	}
}

// parseOrigins reads a comma-separated list of allowed CORS origins, e.g.
// "https://app.example.com,https://www.example.com". Falls back to the local
// Vite dev server so nothing extra is needed for local development.
func parseOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{"http://localhost:5173"}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	return origins
}
