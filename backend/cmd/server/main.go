package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/krishnarajivvns/investiq/internal/api/handlers"
	"github.com/krishnarajivvns/investiq/internal/application/services"
	infraadvisor "github.com/krishnarajivvns/investiq/internal/infrastructure/advisor"
	infradb "github.com/krishnarajivvns/investiq/internal/infrastructure/db"
)

// loadEnv reads KEY=VALUE lines from a .env file and sets any that aren't already in the environment.
func loadEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env file is fine
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func main() {
	loadEnv(".env")
	ctx := context.Background()

	mongoClient, err := infradb.Connect(ctx)
	if err != nil {
		log.Fatalf("MongoDB connection failed: %v", err)
	}
	defer mongoClient.Disconnect(ctx) //nolint:errcheck

	database := mongoClient.Database("investiq")

	profileRepo := infradb.NewMongoProfileRepository(database)

	advisor, err := infraadvisor.NewAdvisor()
	if err != nil {
		log.Fatalf("advisor init failed: %v", err)
	}

	recommendSvc := services.NewRecommendationService(advisor, profileRepo)

	mux := http.NewServeMux()
	mux.Handle("/recommend", handlers.NewRecommendHandler(recommendSvc))
	mux.Handle("/users/profile", handlers.NewProfileHandler(profileRepo))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("  InvestIQ backend running on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
