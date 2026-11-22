package main

import (
	"context"
	"log"

	"github.com/joho/godotenv"

	"jobyuk/internal/config"
	"jobyuk/internal/repository"
	"jobyuk/internal/services"
	"jobyuk/internal/services/reed"
)

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config load failed: %v", err)
	}

	ctx := context.Background()
	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("DB connect failed: %v", err)
	}
	defer db.Close()

	rdb, err := services.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Redis connect failed: %v", err)
	}
	defer rdb.Close()

	repos := repository.NewRepositories(db)
	cache := services.NewCacheService(rdb)
	taxonomy := services.NewTaxonomyService(repos, cache)
	client := reed.NewClient(cfg.ReedAPIKey, cache)
	googleIndexingService := services.NewGoogleIndexingService(cfg.GoogleCredentialsJSON)

	syncService := reed.NewSyncService(client, repos, taxonomy, googleIndexingService, cfg.BaseURL)
	log.Println("🚀 Starting instant sync of UK jobs from Reed API...")
	syncService.RunSync(ctx)
	log.Println("🎉 Sync completed successfully!")
}

