// dbreset prepares and validates a new seed in an isolated schema, backs up the
// configured database, then replaces it. The default mode is read-only.
package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"student-management/config"
	"student-management/seed"
	"time"
)

var safeName = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	execute := flag.Bool("execute", false, "replace the database after backup and validation")
	confirmed := flag.String("database", "", "must exactly match DB_NAME for an executed reset")
	date := flag.String("date", "", "reference school date, YYYY-MM-DD; defaults to today")
	bin := flag.String("mysql-bin", "", "directory containing mysql and mysqldump")
	backupDir := flag.String("backup-dir", "backups", "private backup and demo account directory")
	flag.Parse()
	_ = godotenv.Load()
	name := config.GetEnv("DB_NAME", "student_manager_db")
	if !safeName.MatchString(name) || len(name) > 64 {
		return fmt.Errorf("unsupported database name")
	}
	for _, system := range []string{"mysql", "sys", "performance_schema", "information_schema"} {
		if strings.EqualFold(name, system) {
			return fmt.Errorf("refusing to reset a system schema")
		}
	}
	if *execute && (*confirmed != name || os.Getenv("APP_ENV") == "production" || os.Getenv("GIN_MODE") == "release") {
		return fmt.Errorf("reset requires -database to match DB_NAME and a development environment")
	}
	cfg := mysql.NewConfig()
	cfg.User = config.GetEnv("DB_USER", "root")
	cfg.Passwd = os.Getenv("DB_PASSWORD")
	cfg.Net = "tcp"
	cfg.Addr = config.GetEnv("DB_HOST", "localhost") + ":" + config.GetEnv("DB_PORT", "3306")
	cfg.ParseTime = true
	cfg.Loc = time.FixedZone("Asia/Ho_Chi_Minh", 7*3600)
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}
	defer admin.Close()
	if err = admin.Ping(); err != nil {
		return fmt.Errorf("cannot connect to configured MySQL: %w", err)
	}
	var exists int
	if err = admin.QueryRow("SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name=?", name).Scan(&exists); err != nil {
		return err
	}
	if exists != 1 {
		return fmt.Errorf("configured database %s does not exist; nothing was reset", name)
	}
	var tables int
	if err = admin.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=?", name).Scan(&tables); err != nil {
		return err
	}
	fmt.Printf("Target: %s at %s (%d tables).\n", name, cfg.Addr, tables)
	if !*execute {
		fmt.Println("Read-only preview. To reset, pass -execute -database with the exact target name. No changes were made.")
		return nil
	}
	dumpTool, err := findTool(*bin, "mysqldump")
	if err != nil {
		return err
	}
	mysqlTool, err := findTool(*bin, "mysql")
	if err != nil {
		return err
	}
	now := time.Now().In(cfg.Loc)
	if *date != "" {
		now, err = time.ParseInLocation("2006-01-02", *date, cfg.Loc)
		if err != nil {
			return err
		}
	}
	password := os.Getenv("SEED_PASSWORD")
	if password == "" {
		password = "123456"
	}
	random := make([]byte, 5)
	if _, err = rand.Read(random); err != nil {
		return err
	}
	suffix := time.Now().UTC().Format("20060102T150405") + "_" + hex.EncodeToString(random)
	stage := "__pp_seed_" + suffix
	if _, err = admin.Exec("CREATE DATABASE `" + stage + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		return err
	}
	defer func() {
		if safeName.MatchString(stage) && strings.HasPrefix(stage, "__pp_seed_") && stage != name {
			if _, e := admin.Exec("DROP DATABASE IF EXISTS `" + stage + "`"); e != nil {
				log.Printf("Temporary schema cleanup failed for %s", stage)
			}
		}
	}()
	stagedCfg := *cfg
	stagedCfg.DBName = stage
	stagedDB, err := gorm.Open(gormmysql.Open(stagedCfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	stagedSQL, err := stagedDB.DB()
	if err != nil {
		return err
	}
	defer stagedSQL.Close()
	config.DB = stagedDB
	config.AutoMigrateDB()
	if err = seed.SeedDatabase(stagedDB, seed.Options{Now: now, Password: password}); err != nil {
		return fmt.Errorf("staging seed failed; target unchanged: %w", err)
	}
	summary, err := seed.Validate(stagedDB, now)
	if err != nil {
		return fmt.Errorf("staging validation failed; target unchanged: %w", err)
	}
	fmt.Println("Staging dataset validated successfully.")
	backupRoot, err := filepath.Abs(*backupDir)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(backupRoot, 0700); err != nil {
		return err
	}
	backup := filepath.Join(backupRoot, name+"_"+suffix+".sql")
	stagedDump := filepath.Join(backupRoot, "new_seed_"+suffix+".sql")
	if err = dump(dumpTool, cfg, name, backup); err != nil {
		return fmt.Errorf("backup failed; target unchanged: %w", err)
	}
	info, err := os.Stat(backup)
	if err != nil || info.Size() == 0 {
		return fmt.Errorf("backup is missing or empty; target unchanged")
	}
	file, err := os.Open(backup)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	file.Close()
	if err != nil {
		return err
	}
	if err = dump(dumpTool, cfg, stage, stagedDump); err != nil {
		return fmt.Errorf("staged export failed; target unchanged: %w", err)
	}
	var schemaName, originalCreate string
	if err = admin.QueryRow("SHOW CREATE DATABASE `"+name+"`").Scan(&schemaName, &originalCreate); err != nil {
		return err
	}
	manifest := map[string]interface{}{"database": name, "host": cfg.Addr, "backup": backup, "sha256": hex.EncodeToString(hash.Sum(nil)), "referenceDate": summary.ReferenceDate, "newDataset": summary, "createdAt": time.Now().UTC()}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	if err = os.WriteFile(filepath.Join(backupRoot, "reset_"+suffix+".json"), data, 0600); err != nil {
		return err
	}
	accounts := fmt.Sprintf("PP Academy development demo accounts\nReference date: %s\nShared demo password: %s\n\nAdmin: admin (admin@gmail.com)\nTeacher email login: username@gmail.com\nStudent email: initials.major00001@ppacademy.edu.vn (see student profile)\nTeachers: teacher01 through teacher20\nStudents: student001 through student480\n\nFirst-password-change demos: teacher04, student012\nOther demo accounts can log in directly. All accounts are fictional.\nSQL backup: %s\n", summary.ReferenceDate, password, backup)
	accountFile := filepath.Join(backupRoot, "accounts_"+suffix+".txt")
	if err = os.WriteFile(accountFile, []byte(accounts), 0600); err != nil {
		return err
	}
	fmt.Printf("Backup ready: %s\n", backup)
	rollback := func(cause error) error {
		_, dropErr := admin.Exec("DROP DATABASE IF EXISTS `" + name + "`")
		if dropErr == nil {
			_, dropErr = admin.Exec(originalCreate)
		}
		if dropErr == nil {
			dropErr = importSQL(mysqlTool, cfg, name, backup)
		}
		if dropErr != nil {
			return fmt.Errorf("reset failed (%v); automatic restore failed (%v); retain backup %s", cause, dropErr, backup)
		}
		return fmt.Errorf("reset failed; original database restored: %w", cause)
	}
	if _, err = admin.Exec("DROP DATABASE `" + name + "`"); err != nil {
		return fmt.Errorf("cannot replace target; backup retained: %w", err)
	}
	if _, err = admin.Exec(originalCreate); err != nil {
		return rollback(err)
	}
	if err = importSQL(mysqlTool, cfg, name, stagedDump); err != nil {
		return rollback(err)
	}
	finalCfg := *cfg
	finalCfg.DBName = name
	finalDB, err := gorm.Open(gormmysql.Open(finalCfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return rollback(err)
	}
	finalSQL, err := finalDB.DB()
	if err != nil {
		return rollback(err)
	}
	defer finalSQL.Close()
	result, err := seed.Validate(finalDB, now)
	if err != nil {
		return rollback(err)
	}
	for key, value := range summary.Counts {
		if result.Counts[key] != value {
			return rollback(fmt.Errorf("row count mismatch for %s", key))
		}
	}
	output, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("Reset completed and verified.\n%s\nDemo account credentials: %s\n", output, accountFile)
	return nil
}
func findTool(directory, name string) (string, error) {
	if directory != "" {
		for _, file := range []string{name + ".exe", name} {
			path := filepath.Join(directory, file)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, nil
			}
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("cannot find %s; provide -mysql-bin", name)
	}
	return path, nil
}
func mysqlCommand(tool string, cfg *mysql.Config, args ...string) *exec.Cmd {
	base := []string{"--host=" + strings.Split(cfg.Addr, ":")[0], "--port=" + strings.Split(cfg.Addr, ":")[1], "--user=" + cfg.User, "--default-character-set=utf8mb4"}
	cmd := exec.Command(tool, append(base, args...)...)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(value), "MYSQL_PWD=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	cmd.Env = append(cmd.Env, "MYSQL_PWD="+cfg.Passwd)
	return cmd
}
func dump(tool string, cfg *mysql.Config, name, path string) error {
	cmd := mysqlCommand(tool, cfg, "--single-transaction", "--routines", "--events", "--triggers", "--hex-blob", "--set-gtid-purged=OFF", "--no-tablespaces", "--result-file="+path, name)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mysqldump failed: %s", string(output))
	}
	return os.Chmod(path, 0600)
}
func importSQL(tool string, cfg *mysql.Config, name, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	cmd := mysqlCommand(tool, cfg, name)
	cmd.Stdin = file
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mysql import failed: %s", string(output))
	}
	return nil
}
