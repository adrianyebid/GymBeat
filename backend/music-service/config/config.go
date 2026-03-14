package config

import "os"

// Config contiene la configuración del servicio
type Config struct {
	Port string
}

// Load carga la configuración desde variables de entorno con valores por defecto
func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	return &Config{
		Port: port,
	}
}
