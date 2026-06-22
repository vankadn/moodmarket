// cmd/cleanup-phantom-decisions/main.go
//
// One-time cleanup for phantom "recommend-only" decision documents written by
// the /recommend path before the Phase 26 fix. These docs have no decision_type,
// no config_id, and empty receipts — they were never real investment events.
//
// Usage:
//
//	go run ./cmd/cleanup-phantom-decisions          # dry-run: prints count + IDs
//	go run ./cmd/cleanup-phantom-decisions --confirm # deletes
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	confirm := flag.Bool("confirm", false, "actually delete the phantom documents")
	flag.Parse()

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Disconnect(ctx)

	coll := client.Database("investiq").Collection("decisions")

	// Phantom docs: decision_type absent or empty, config_id absent or empty,
	// and receipts array absent or zero-length.
	filter := bson.M{
		"$and": bson.A{
			bson.M{"$or": bson.A{
				bson.M{"decision_type": bson.M{"$exists": false}},
				bson.M{"decision_type": ""},
			}},
			bson.M{"$or": bson.A{
				bson.M{"config_id": bson.M{"$exists": false}},
				bson.M{"config_id": ""},
			}},
			bson.M{"$or": bson.A{
				bson.M{"receipts": bson.M{"$exists": false}},
				bson.M{"receipts": bson.M{"$size": 0}},
			}},
		},
	}

	type minDoc struct {
		ID        primitive.ObjectID `bson:"_id"`
		UserID    string             `bson:"user_id"`
		Timestamp time.Time          `bson:"timestamp"`
	}

	cursor, err := coll.Find(ctx, filter, options.Find().SetProjection(bson.M{
		"_id": 1, "user_id": 1, "timestamp": 1,
	}))
	if err != nil {
		log.Fatalf("find: %v", err)
	}
	var docs []minDoc
	if err := cursor.All(ctx, &docs); err != nil {
		log.Fatalf("decode: %v", err)
	}

	fmt.Printf("phantom documents found: %d\n", len(docs))
	for _, d := range docs {
		fmt.Printf("  id=%s  user=%s  timestamp=%s\n", d.ID.Hex(), d.UserID, d.Timestamp.Format(time.RFC3339))
	}

	if len(docs) == 0 {
		fmt.Println("nothing to delete")
		return
	}

	if !*confirm {
		fmt.Println("\ndry-run — rerun with --confirm to delete")
		return
	}

	ids := make([]primitive.ObjectID, len(docs))
	for i, d := range docs {
		ids[i] = d.ID
	}
	res, err := coll.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		log.Fatalf("delete: %v", err)
	}
	fmt.Printf("deleted %d phantom documents\n", res.DeletedCount)
}
