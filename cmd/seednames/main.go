// seednames removes the redundant major suffix from seeded teacher names.
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
	// Match the exact suffix against the teacher's major so custom names stay intact.
	result := config.DB.Exec(`UPDATE users u
		JOIN teachers t ON t.user_id = u.id
		JOIN majors m ON m.id = t.major_id
		SET u.full_name = LEFT(u.full_name, CHAR_LENGTH(u.full_name) - CHAR_LENGTH(CONCAT(' – ', m.name)))
		WHERE RIGHT(u.full_name, CHAR_LENGTH(CONCAT(' – ', m.name))) = CONCAT(' – ', m.name)`)
	if result.Error != nil {
		log.Fatal(result.Error)
	}
	var remaining int64
	if err := config.DB.Raw(`SELECT COUNT(*) FROM users u JOIN teachers t ON t.user_id=u.id JOIN majors m ON m.id=t.major_id WHERE RIGHT(u.full_name, CHAR_LENGTH(CONCAT(' – ', m.name)))=CONCAT(' – ', m.name)`).Scan(&remaining).Error; err != nil {
		log.Fatal(err)
	}
	if remaining != 0 {
		log.Fatal("teacher name verification failed")
	}
	fmt.Printf("Updated %d teacher names; no redundant major suffixes remain.\n", result.RowsAffected)
}
