package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var Client *mongo.Client

func ConnectDB() *mongo.Client {
	// Replace <db_password> with your actual password
	uri := "mongodb+srv://rksah:<db_password>@cluster0.kac1fxd.mongodb.net/?retryWrites=true&w=majority&appName=Cluster0"
	// Use the SetServerAPIOptions() method to set the version of the Stable API
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a new client and connect to the server
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
	}

	// Send a ping to confirm a successful connection
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatal("Could not ping MongoDB:", err)
	}

	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")

	Client = client
	return Client
}