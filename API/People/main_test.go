package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func setupTestDB(t *testing.T) *mongo.Client {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		t.Skip("MONGODB_URI not set, skipping integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(mongoURI).SetServerAPIOptions(serverAPI)
	
	testClient, err := mongo.Connect(opts)
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	if err := testClient.Ping(ctx, nil); err != nil {
		t.Fatalf("Failed to ping MongoDB: %v", err)
	}

	return testClient
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})
	
	r.POST("/people", createPersonHandler)
	r.GET("/people", getAllPeopleHandler)
	r.GET("/people/:id", getPersonHandler)
	
	return r
}

func TestHealthEndpoint(t *testing.T) {
	router := setupRouter()
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var response map[string]string
	json.Unmarshal(w.Body.Bytes(), &response)
	
	if response["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response["status"])
	}
}

func TestCreatePerson(t *testing.T) {
	testClient := setupTestDB(t)
	defer testClient.Disconnect(context.Background())
	
	client = testClient
	peopleColl = client.Database("isfdb_test").Collection("people")
	
	// Clean up before test
	ctx := context.Background()
	peopleColl.DeleteMany(ctx, bson.M{})
	
	router := setupRouter()
	
	person := Person{
		Name:  "John Doe",
		Email: "john@example.com",
	}
	
	jsonData, _ := json.Marshal(person)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/people", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}
	
	var createdPerson Person
	json.Unmarshal(w.Body.Bytes(), &createdPerson)
	
	if createdPerson.Name != person.Name {
		t.Errorf("Expected name '%s', got '%s'", person.Name, createdPerson.Name)
	}
	
	// Clean up after test
	peopleColl.DeleteMany(ctx, bson.M{})
}

func TestCreatePersonInvalidEmail(t *testing.T) {
	testClient := setupTestDB(t)
	defer testClient.Disconnect(context.Background())
	
	client = testClient
	peopleColl = client.Database("isfdb_test").Collection("people")
	
	router := setupRouter()
	
	person := Person{
		Name:  "Jane Doe",
		Email: "invalid-email",
	}
	
	jsonData, _ := json.Marshal(person)
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/people", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetAllPeople(t *testing.T) {
	testClient := setupTestDB(t)
	defer testClient.Disconnect(context.Background())
	
	client = testClient
	peopleColl = client.Database("isfdb_test").Collection("people")
	
	ctx := context.Background()
	peopleColl.DeleteMany(ctx, bson.M{})
	
	// Insert test data
	testPeople := []interface{}{
		Person{ID: bson.NewObjectID(), Name: "Alice", Email: "alice@example.com"},
		Person{ID: bson.NewObjectID(), Name: "Bob", Email: "bob@example.com"},
	}
	peopleColl.InsertMany(ctx, testPeople)
	
	router := setupRouter()
	
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/people", nil)
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	
	var people []Person
	json.Unmarshal(w.Body.Bytes(), &people)
	
	if len(people) != 2 {
		t.Errorf("Expected 2 people, got %d", len(people))
	}
	
	// Clean up
	peopleColl.DeleteMany(ctx, bson.M{})
}