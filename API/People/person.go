package main

import (
	"go.mongodb.org/mongo-driver/v2/bson"
	"regexp"
)

type Person struct {
	ID bson.ObjectID
	Name string
	Email string
}

func (p *Person) isValidEmail() bool {
	re := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	return re.MatchString(p.Email)
}

