package main

import (
	"fmt"
	"student-management/config"
	"student-management/seed"
)

func main() {

	config.ConnectDB()

	seed.SeedData()

	fmt.Println("Seeding data completed successfully!")
}
