package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func InitDB() (*sql.DB, error) {
	appPath, err := os.Getwd()
	if err != nil {
		log.Fatal("Ошибка при получении рабочей директории: ", err)
	}

	dbFile := filepath.Join(appPath, "data/scheduler.db")

	_, err = os.Stat(dbFile)

	var install bool
	if err != nil {
		install = true
	}

	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		log.Fatal("Ошибка при открытии базы данных: ", err)
	}

	if install {
		if err := createTables(db); err != nil {
			log.Fatal("Ошибка при создании базы данных: ", err)
		}
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	sqlCreateTable := `
    CREATE TABLE IF NOT EXISTS scheduler (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        date TEXT NOT NULL,
        title TEXT NOT NULL,
        comment TEXT,
        repeat TEXT CHECK(length(repeat) <= 128)
    );`

	sqlCreateIndex := `
	CREATE INDEX IF NOT EXISTS idx_scheduler_date ON scheduler(date);`

	_, err := db.Exec(sqlCreateTable)
	if err != nil {
		return fmt.Errorf("ошибка при создании таблицы: %w", err)
	}
	fmt.Println("Таблица scheduler успешно создана")

	_, err = db.Exec(sqlCreateIndex)
	if err != nil {
		return fmt.Errorf("ошибка при создании индекса: %w", err)
	}
	fmt.Println("Индекс по столбцу date успешно создан")

	return nil
}
