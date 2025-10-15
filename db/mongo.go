package db

import (
	"context"
	"fmt"


	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var Client *mongo.Client

func ConnectDB() *mongo.Client {
	uri := "mongodb+srv://rksah:isfsupporters@cluster0.kac1fxd.mongodb.net/?retryWrites=true&w=majority&appName=Cluster0"

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(uri).SetServerAPIOptions(serverAPI)
	client, err := mongo.Connect(context.TODO(), opts)
	if err != nil {
    panic(err)
	}
	defer func() {
	if err = client.Disconnect(context.TODO()); err != nil {
		panic(err)
	}
	}()
	// Send a ping to confirm a successful connection
	if err := client.Ping(context.TODO(), readpref.Primary()); err != nil {
	panic(err)
	}
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")
	Client = client
	return Client
}
