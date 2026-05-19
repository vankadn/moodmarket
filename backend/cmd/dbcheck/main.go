// dbcheck is a one-shot diagnostic tool. Run it to inspect MongoDB collections.
// Usage:
//
//	go run ./cmd/dbcheck                          # ticker_classifications (default)
//	go run ./cmd/dbcheck decisions                # investment_decisions
//	go run ./cmd/dbcheck profiles                 # user profiles
//
// Reads MONGODB_URI from the environment or .env file (same as the server).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	loadEnv(".env")

	collection := "ticker_classifications"
	if len(os.Args) > 1 {
		collection = os.Args[1]
	}

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Disconnect(ctx) //nolint:errcheck

	coll := client.Database("investiq").Collection(collection)

	total, err := coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		log.Fatalf("count: %v", err)
	}
	fmt.Printf("Collection: %s   Total docs: %d\n\n", collection, total)

	cursor, err := coll.Find(ctx, bson.M{}, options.Find().SetLimit(200))
	if err != nil {
		log.Fatalf("find: %v", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		log.Fatalf("decode: %v", err)
	}

	for _, doc := range results {
		delete(doc, "_id") // omit ObjectID noise
		b, _ := json.MarshalIndent(doc, "", "  ")
		fmt.Println(string(b))
	}
}

func loadEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
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
