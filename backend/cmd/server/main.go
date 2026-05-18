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
	"github.com/krishnarajivvns/investiq/internal/api/router"
	"github.com/krishnarajivvns/investiq/internal/application/scheduler"
	"github.com/krishnarajivvns/investiq/internal/application/services"
	infraadvisor "github.com/krishnarajivvns/investiq/internal/infrastructure/advisor"
	infraauth "github.com/krishnarajivvns/investiq/internal/infrastructure/auth"
	infrabanking "github.com/krishnarajivvns/investiq/internal/infrastructure/banking"
	inflabrokerage "github.com/krishnarajivvns/investiq/internal/infrastructure/brokerage"
	infradb "github.com/krishnarajivvns/investiq/internal/infrastructure/db"
	infracalendar "github.com/krishnarajivvns/investiq/internal/infrastructure/calendar"
	infraextractor "github.com/krishnarajivvns/investiq/internal/infrastructure/extractor"
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
		os.Setenv("DOCUMENT_EXTRACTOR", "mock")
		os.Setenv("MARKET_CALENDAR", "mock")
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

	newsProvider, err := infranews.NewNewsProvider()
	if err != nil {
		log.Fatalf("news provider init failed: %v", err)
	}

	advisor, err := infraadvisor.NewAdvisor(newsProvider)
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

	authProvider, err := infraauth.NewAuthProvider()
	if err != nil {
		log.Fatalf("auth provider init failed: %v", err)
	}

	schedulerRepo := infradb.NewMongoSchedulerRepository(database)
	autoInvestRepo := infradb.NewMongoAutoInvestRepository(database)
	notificationProvider := infranotifications.NewNotificationProvider()

	marketCalendar, err := infracalendar.NewMarketCalendar()
	if err != nil {
		log.Fatalf("market calendar init failed: %v", err)
	}

	documentExtractor, err := infraextractor.NewDocumentExtractor()
	if err != nil {
		log.Fatalf("document extractor init failed: %v", err)
	}
	documentRepo := infradb.NewMongoDocumentRepository(database)

	recommendSvc := services.NewRecommendationService(advisor, profileRepo, marketProvider, decisionRepo, financialDataProvider, brokerageFactory, documentRepo)
	investSvc := services.NewInvestmentService(brokerageFactory, profileRepo, decisionRepo, marketProvider)
	documentSvc := services.NewDocumentService(documentExtractor, documentRepo)
	idp := middleware.ContextIdentityProvider{}

	autoInvestScheduler := scheduler.NewAutoInvestScheduler(autoInvestRepo, profileRepo, recommendSvc, investSvc, schedulerRepo, notificationProvider, marketCalendar)
	go autoInvestScheduler.Start(ctx)

	plaidHandler := handlers.NewPlaidHandler(financialDataProvider, profileRepo, idp)
	activityHandler := handlers.NewActivityHandler(idp, decisionRepo)
	evalHandler := handlers.NewEvalHandler(idp, decisionRepo)
	orderHandler := handlers.NewOrderHandler(idp, profileRepo, brokerageFactory)
	documentHandler := handlers.NewDocumentHandler(documentSvc, idp)

	h := router.Handlers{
		DevLogin:             handlers.NewDevLoginHandler(),
		Recommend:            handlers.NewRecommendHandler(recommendSvc, idp),
		Invest:               handlers.NewInvestHandler(investSvc, idp, profileRepo, notificationProvider),
		Profile:              handlers.NewProfileHandler(profileRepo, idp),
		Plaid:                plaidHandler,
		AutoInvest:           handlers.NewAutoInvestConfigHandler(autoInvestRepo, idp),
		AutoInvestConfigs:    handlers.NewAutoInvestConfigsHandler(autoInvestRepo, idp),
		CashContext:          handlers.NewCashContextHandler(recommendSvc, idp),
		Activity:             http.HandlerFunc(activityHandler.GetActivity),
		ActivityByStrategy:   http.HandlerFunc(activityHandler.GetActivityByStrategy),
		ActivityStrategyPnL:  handlers.NewStrategyPnLHandler(idp, decisionRepo, profileRepo, brokerageFactory),
		Brokerage:            handlers.NewBrokerageHandler(profileRepo, idp),
		BrokerageConnections: handlers.NewBrokerageConnectionsHandler(profileRepo, idp),
		Notifications:        handlers.NewNotificationSettingsHandler(profileRepo, idp),
		Portfolio:            handlers.NewPortfolioHandler(profileRepo, brokerageFactory, idp),
		Order:                http.HandlerFunc(orderHandler.GetOrder),
		Document:             documentHandler,
		Docs:                 handlers.NewDocsHandler(),
		Eval:                 evalHandler,
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("  InvestIQ backend running on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, router.Build(h, authProvider)))
}
