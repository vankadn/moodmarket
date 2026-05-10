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
	"github.com/krishnarajivvns/investiq/internal/application/scheduler"
	"github.com/krishnarajivvns/investiq/internal/application/services"
	infraadvisor "github.com/krishnarajivvns/investiq/internal/infrastructure/advisor"
	infraauth "github.com/krishnarajivvns/investiq/internal/infrastructure/auth"
	infrabanking "github.com/krishnarajivvns/investiq/internal/infrastructure/banking"
	inflabrokerage "github.com/krishnarajivvns/investiq/internal/infrastructure/brokerage"
	infradb "github.com/krishnarajivvns/investiq/internal/infrastructure/db"
	inframarket "github.com/krishnarajivvns/investiq/internal/infrastructure/market"
	infranews "github.com/krishnarajivvns/investiq/internal/infrastructure/news"
	infranotifications "github.com/krishnarajivvns/investiq/internal/infrastructure/notifications"
	"github.com/krishnarajivvns/investiq/internal/middleware"
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
			os.Setenv(key, strings.Trim(value, `"'`))
		}
	}
}

func main() {
	loadEnv(".env")

	if os.Getenv("MOCK_ALL") == "true" {
		log.Println("[config] MOCK_ALL=true — overriding all providers to mock")
		os.Setenv("AI_PROVIDER", "mock")
		os.Setenv("MARKET_PROVIDER", "mock")
		os.Setenv("FINANCIAL_DATA_PROVIDER", "mock")
		os.Setenv("BROKERAGE_PROVIDER", "mock")
		os.Setenv("NEWS_PROVIDER", "mock")
		os.Setenv("DEV_MODE", "true")
	}

	ctx := context.Background()

	mongoClient, err := infradb.Connect(ctx)
	if err != nil {
		log.Fatalf("MongoDB connection failed: %v", err)
	}
	defer mongoClient.Disconnect(ctx) //nolint:errcheck

	database := mongoClient.Database("investiq")
	profileRepo := infradb.NewMongoProfileRepository(database)
	decisionRepo := infradb.NewMongoDecisionRepository(database)

	advisor, err := infraadvisor.NewAdvisor()
	if err != nil {
		log.Fatalf("advisor init failed: %v", err)
	}

	marketProvider, err := inframarket.NewMarketDataProvider()
	if err != nil {
		log.Fatalf("market data provider init failed: %v", err)
	}

	brokerageFactory, err := inflabrokerage.NewBrokerageFactory()
	if err != nil {
		log.Fatalf("brokerage factory init failed: %v", err)
	}

	financialDataProvider, err := infrabanking.NewFinancialDataProvider()
	if err != nil {
		log.Fatalf("financial data provider init failed: %v", err)
	}

	newsProvider, err := infranews.NewNewsProvider()
	if err != nil {
		log.Fatalf("news provider init failed: %v", err)
	}

	authProvider, err := infraauth.NewAuthProvider()
	if err != nil {
		log.Fatalf("auth provider init failed: %v", err)
	}

	schedulerRepo := infradb.NewMongoSchedulerRepository(database)
	autoInvestRepo := infradb.NewMongoAutoInvestRepository(database)
	notificationProvider := infranotifications.NewNotificationProvider()

	recommendSvc := services.NewRecommendationService(advisor, profileRepo, marketProvider, decisionRepo, financialDataProvider, brokerageFactory, newsProvider)
	investSvc := services.NewInvestmentService(brokerageFactory, profileRepo, decisionRepo, marketProvider)
	idp := middleware.ContextIdentityProvider{}

	autoInvestScheduler := scheduler.NewAutoInvestScheduler(autoInvestRepo, recommendSvc, investSvc, schedulerRepo, notificationProvider)
	go autoInvestScheduler.Start(ctx)

	plaidHandler := handlers.NewPlaidHandler(financialDataProvider, profileRepo, idp)

	mux := http.NewServeMux()
	mux.Handle("/auth/dev-login", handlers.NewDevLoginHandler())
	mux.Handle("/recommend", handlers.NewRecommendHandler(recommendSvc, idp))
	mux.Handle("/invest", handlers.NewInvestHandler(investSvc, idp))
	mux.Handle("/users/profile", handlers.NewProfileHandler(profileRepo, idp))
	mux.Handle("/plaid/link-token", plaidHandler)
	mux.Handle("/plaid/exchange", plaidHandler)
	mux.Handle("/plaid/accounts/", plaidHandler) // trailing slash = prefix match for /{item_id}
	mux.Handle("/users/auto-invest/config", handlers.NewAutoInvestConfigHandler(autoInvestRepo, idp))
	mux.Handle("/users/cash-context", handlers.NewCashContextHandler(recommendSvc, idp))
	activityHandler := handlers.NewActivityHandler(idp, decisionRepo)
	mux.HandleFunc("/users/activity", activityHandler.GetActivity)
	brokerageHandler := handlers.NewBrokerageHandler(profileRepo, idp)
	mux.Handle("/brokerage/connect", brokerageHandler)

	orderHandler := handlers.NewOrderHandler(idp, profileRepo, brokerageFactory)
	mux.HandleFunc("/orders/", orderHandler.GetOrder)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// topMux registers /health without auth so Railway / Docker healthchecks work.
	// Everything else falls through to the full middleware chain.
	topMux := http.NewServeMux()
	topMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	topMux.Handle("/", middleware.CORS(middleware.UserIdentity(authProvider, mux)))

	fmt.Printf("  InvestIQ backend running on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, topMux))
}
