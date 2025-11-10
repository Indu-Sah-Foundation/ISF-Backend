package main

import (
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Person struct {
	ID    bson.ObjectID `json:"id" bson:"_id,omitempty"`
	Name  string        `json:"name" bson:"name"`
	Email string        `json:"email" bson:"email"`
}

func (p *Person) isValidEmail() bool {
	if p.Email == "" {
		return false
	}
	// Simple email validation regex
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(strings.ToLower(p.Email))
}
