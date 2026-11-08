// seedemails updates only the previous sample email domains without resetting data.
package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"log"
	"student-management/config"
)

func main() {
	_ = godotenv.Load()
	config.ConnectDB()
	var updated int64
	err := config.DB.Transaction(func(tx *gorm.DB) error {
		r := tx.Exec("UPDATE users SET email = CONCAT(username, '@gmail.com') WHERE email = CONCAT(username, '@ppacademy.example')")
		if r.Error != nil {
			return r.Error
		}
		updated = r.RowsAffected
		if err := tx.Exec("UPDATE site_settings SET contact_email = 'daotao@gmail.com' WHERE contact_email = 'daotao@ppacademy.example'").Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE consultations SET email = 'tuvan.ppacademy@gmail.com' WHERE email = 'tuvan-mau@example.test'").Error
	})
	if err != nil {
		log.Fatal(err)
	}
	var count int64
	if err := config.DB.Table("users").Where("username = ? AND email = ?", "admin", "admin@gmail.com").Count(&count).Error; err != nil {
		log.Fatal(err)
	}
	if count != 1 {
		log.Fatal("admin email verification failed")
	}
	fmt.Printf("Updated %d sample account emails; admin@gmail.com verified. Passwords unchanged.\n", updated)
}
