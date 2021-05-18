package main

import (
	"github.com/redhatinsights/edge-api/pkg/commits"
	"github.com/redhatinsights/edge-api/pkg/db"
)

func main() {
	db.InitDB()
	err := db.DB.AutoMigrate(&commits.Commit{})
	if err != nil {
		panic(err)
	}
}
