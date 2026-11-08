// seedpassword changes passwords of existing seed accounts without resetting school data.
package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"log"
	"student-management/config"
	"student-management/models"
)

func main() {
	_ = godotenv.Load()
	config.ConnectDB()
	usernames := []string{"admin"}
	for i := 1; i <= 20; i++ {
		usernames = append(usernames, fmt.Sprintf("teacher%02d", i))
	}
	for i := 1; i <= 480; i++ {
		usernames = append(usernames, fmt.Sprintf("student%03d", i))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("123456"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	var updated int64
	err = config.DB.Transaction(func(tx *gorm.DB) error {
		var ids []uint
		if err := tx.Model(&models.User{}).Where("username IN ? AND email = CONCAT(username, '@gmail.com')", usernames).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return fmt.Errorf("no matching seed accounts")
		}
		r := tx.Model(&models.User{}).Where("id IN ?", ids).Update("password", string(hash))
		if r.Error != nil {
			return r.Error
		}
		updated = r.RowsAffected
		return tx.Where("user_id IN ?", ids).Delete(&models.AuthSession{}).Error
	})
	if err != nil {
		log.Fatal(err)
	}
	var admin models.User
	if err := config.DB.Where("username = ?", "admin").First(&admin).Error; err != nil {
		log.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte("123456")); err != nil {
		log.Fatal("password verification failed")
	}
	fmt.Printf("Updated %d seed accounts; admin password verified. Previous sessions revoked.\n", updated)
}
