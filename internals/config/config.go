package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Mongo_db_username          string
	Mongo_db_password          string
	Mongo_db_connection_string string
	JWT_secret                 string
}

func GetConfig() *Config {
	err := godotenv.Load()

	if err != nil {
		log.Fatalf("Error loading .env file %v", err)
	}

	return &Config{
		Mongo_db_username:          os.Getenv("mongo_db_username"),
		Mongo_db_password:          os.Getenv("mongo_db_password"),
		Mongo_db_connection_string: os.Getenv("mongo_connection_string"),
		JWT_secret:                 os.Getenv("jwt_secret"),
	}
}
