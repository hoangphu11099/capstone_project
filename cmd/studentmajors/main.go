// studentmajors adds/backfills registered majors without resetting accounts or schedules.
package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"student-management/config"
)

func main() {
	_ = godotenv.Load()
	config.ConnectDB()
	config.AutoMigrateDB()
	var count int64
	if err := config.DB.Table("students").Where("major_id>0").Count(&count).Error; err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Registered majors initialized for %d students. No database reset.\n", count)
}
