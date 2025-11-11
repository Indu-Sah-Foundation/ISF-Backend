package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

var client *mongo.Client
var peopleColl *mongo.Collection

func main() {
	// Get MongoDB URI from environment variable (set by Azure App Settings)
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Fatal("MONGODB_URI environment variable is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(mongoURI).SetServerAPIOptions(serverAPI)
	var err error
	client, err := mongo.Connect(opts)
	if err != nil {
		log.Fatal("mongo.Connect: ", err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal("Disconnect Error: ", err)
		}
	}()
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Fatal("Mongo Ping Failed: ", err)
	}
	log.Println("Connected to MongoDB server! ")
	peopleColl = client.Database("isfdb").Collection("people")

	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.POST("/people", createPersonHandler)
	r.GET("/people", getAllPeopleHandler)
	r.GET("/people/:id", getPersonHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Printf("API listening on :%s\n", port)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exiting")

}

func createPersonHandler(c *gin.Context) {
	var p Person
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
		return
	}

	if !p.isValidEmail() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := peopleColl.InsertOne(ctx, p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert failed: " + err.Error()})
		return
	}

	insertedID := res.InsertedID.(bson.ObjectID)
	p.ID = insertedID
	c.JSON(http.StatusCreated, p)
}

func getAllPeopleHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := peopleColl.Find(ctx, bson.M{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed: " + err.Error()})
		return
	}
	defer cursor.Close(ctx)

	var people []Person
	if err = cursor.All(ctx, &people); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decode failed: " + err.Error()})
		return
	}

	// Return empty array instead of null if no people found
	if people == nil {
		people = []Person{}
	}

	c.JSON(http.StatusOK, people)
}

func getPersonHandler(c *gin.Context) {
	id := c.Param("id")
	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var p Person
	err = peopleColl.FindOne(ctx, bson.M{"_id": objID}).Decode(&p)
	if err == mongo.ErrNoDocuments {
		c.JSON(http.StatusNotFound, gin.H{"error": "person not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, p)
}
