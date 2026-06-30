// cmd/cleanup-duplicate-budget-skips/main.go
//
// One-time cleanup for duplicate "daily budget exhausted" skip decisions written
// by the scheduler before the HasSkipToday guard was added. For each
// (user_id, config_id, calendar-day) group, keeps the earliest document and
// deletes every later one.
//
// Usage:
//
//	go run ./cmd/cleanup-duplicate-budget-skips          # dry-run: prints what would be deleted
//	go run ./cmd/cleanup-duplicate-budget-skips --confirm # deletes
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	loadEnv(".env")

	confirm := flag.Bool("confirm", false, "actually delete the duplicate documents")
	flag.Parse()

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Disconnect(ctx)

	coll := client.Database("investiq").Collection("decisions")

	// Pull all budget-exhausted skip docs, sorted oldest-first so we can keep [0] per group.
	filter := bson.M{
		"decision_type": "skip",
		"skip_reason":   "daily budget exhausted for today",
	}
	type minDoc struct {
		ID        primitive.ObjectID `bson:"_id"`
		UserID    string             `bson:"user_id"`
		ConfigID  string             `bson:"config_id"`
		Timestamp time.Time          `bson:"timestamp"`
	}
	cursor, err := coll.Find(ctx, filter,
		options.Find().
			SetSort(bson.D{{Key: "timestamp", Value: 1}}).
			SetProjection(bson.M{"_id": 1, "user_id": 1, "config_id": 1, "timestamp": 1}),
	)
	if err != nil {
		log.Fatalf("find: %v", err)
	}
	var docs []minDoc
	if err := cursor.All(ctx, &docs); err != nil {
		log.Fatalf("decode: %v", err)
	}

	fmt.Printf("total budget-exhausted skip documents: %d\n", len(docs))

	// Group by (user_id, config_id, calendar-date in UTC). The first doc in each
	// group (earliest timestamp) is the keeper; the rest are duplicates.
	type groupKey struct {
		UserID   string
		ConfigID string
		Day      string // "2006-01-02" in UTC
	}
	seen := map[groupKey]bool{}
	var toDelete []primitive.ObjectID

	for _, d := range docs {
		key := groupKey{
			UserID:   d.UserID,
			ConfigID: d.ConfigID,
			Day:      d.Timestamp.UTC().Format("2006-01-02"),
		}
		if seen[key] {
			toDelete = append(toDelete, d.ID)
			fmt.Printf("  duplicate  id=%s  config=%s  timestamp=%s\n",
				d.ID.Hex(), d.ConfigID, d.Timestamp.Format(time.RFC3339))
		} else {
			seen[key] = true
			fmt.Printf("  keeper     id=%s  config=%s  timestamp=%s\n",
				d.ID.Hex(), d.ConfigID, d.Timestamp.Format(time.RFC3339))
		}
	}

	fmt.Printf("\nkeepers: %d   duplicates to delete: %d\n", len(docs)-len(toDelete), len(toDelete))

	if len(toDelete) == 0 {
		fmt.Println("nothing to delete")
		return
	}

	if !*confirm {
		fmt.Println("\ndry-run — rerun with --confirm to delete")
		return
	}

	res, err := coll.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": toDelete}})
	if err != nil {
		log.Fatalf("delete: %v", err)
	}
	fmt.Printf("deleted %d duplicate budget-skip documents\n", res.DeletedCount)
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
