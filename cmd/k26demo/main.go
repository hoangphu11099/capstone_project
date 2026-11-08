package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"log"
	"student-management/config"
	"student-management/seed"
	"time"
)

func main() {
	_ = godotenv.Load()
	config.ConnectDB()
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("UPDATE schedules SET end_time = '10:30' WHERE start_time = '07:30' AND end_time = '11:30'").Error; err != nil {
			return err
		}
		if err := tx.Exec("UPDATE class_offers SET plan_json = REPLACE(plan_json, ?, ?) WHERE plan_json LIKE ?", `"11:30"`, `"10:30"`, `%07:30%`).Error; err != nil {
			return err
		}
		if err := seed.NormalizeCohortSemesters(tx); err != nil {
			return err
		}
		return seed.ConfigureK26Demo(tx, time.Now())
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("K26 demo prepared: 8 classes, recurring weekly patterns, 13:30-16:30 through 2028-08-31.")
	var summary struct {
		Classes   int64
		Lessons   int64
		FirstDate string
		LastDate  string
	}
	if err := config.DB.Raw(`SELECT COUNT(DISTINCT s.class_id) AS classes, COUNT(*) AS lessons,
		CAST(MIN(s.full_date) AS CHAR) AS first_date, CAST(MAX(s.full_date) AS CHAR) AS last_date
		FROM schedules s JOIN classes c ON c.id=s.class_id
		WHERE c.cohort_year=2026 AND s.full_date>=CURRENT_DATE()`).Scan(&summary).Error; err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Verified: %d classes, %d lessons, %s to %s.\n", summary.Classes, summary.Lessons, summary.FirstDate, summary.LastDate)
}
