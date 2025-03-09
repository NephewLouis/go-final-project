package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var task struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Title   string `json:"title"`
	Comment string `json:"comment"`
	Repeat  string `json:"repeat"`
}

const dateFormat = "20060102"

func TaskHandler(database *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			updateTaskHandler(database, w, r)
		case http.MethodGet:
			getTaskHandler(database, w, r)
		case http.MethodPost:
			createTaskHandler(database, w, r)
		case http.MethodDelete:
			deleteTaskHandler(database, w, r)
		default:
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		}
	}
}

func deleteTaskHandler(database *sql.DB, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error":"Метод не поддерживается"}`, http.StatusMethodNotAllowed)
		return
	}

	idParam := r.URL.Query().Get("id")
	if idParam == "" {
		http.Error(w, `{"error":"Не указан идентификатор"}`, http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		http.Error(w, `{"error":"Указан некорректный идентификатор"}`, http.StatusBadRequest)
		log.Println("Указан некорректный идентификатор: ", err)
		return
	}

	res, err := database.Exec("DELETE FROM scheduler WHERE id=?", id)
	if err != nil {
		http.Error(w, `{"error": "Ошибка при удалении задачи"}`, http.StatusInternalServerError)
		log.Println("Ошибка при удалении задачи: ", err)
		return
	}
	cnt, err := res.RowsAffected()
	if err != nil {
		log.Println("Ошибка при получении количества удаленных строк: ", err)
		http.Error(w, `{"error":"Ошибка базы данных"}`, http.StatusInternalServerError)
		return
	}
	if cnt == 0 {
		http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{})
}

func MarkTaskDoneHandler(database *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"Метод не поддерживается"}`, http.StatusMethodNotAllowed)
			return
		}

		idParam := r.URL.Query().Get("id")
		if idParam == "" {
			http.Error(w, `{"error":"Не указан идентификатор"}`, http.StatusBadRequest)
			return
		}

		id, err := strconv.Atoi(idParam)
		if err != nil || id <= 0 {
			http.Error(w, `{"error":"Некорректный идентификатор"}`, http.StatusBadRequest)
			log.Println("Некорректный идентификатор: ", err)
			return
		}

		query := `SELECT id, date, repeat FROM scheduler WHERE id = ?`
		err = database.QueryRow(query, id).Scan(&task.ID, &task.Date, &task.Repeat)
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
			log.Println("Задача не найдена: ", err)
			return
		} else if err != nil {
			http.Error(w, `{"error":"Ошибка при извлечении задачи из базы данных"}`, http.StatusInternalServerError)
			log.Println("Ошибка базы данных: ", err)
			return
		}

		if task.Repeat == "" {
			_, err = database.Exec(`DELETE FROM scheduler WHERE id = ?`, id)
			if err != nil {
				http.Error(w, `{"error":"Ошибка при удалении задачи"}`, http.StatusInternalServerError)
				log.Println("Ошибка при удалении задачи: ", err)
				return
			}
		} else {
			now := time.Now()
			nextDate, err := NextDate(now, task.Date, task.Repeat)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
				log.Println("Ошибка при рассчете даты", err)
				return
			}

			_, err = database.Exec(`UPDATE scheduler SET date = ? WHERE id = ?`, nextDate, id)
			if err != nil {
				http.Error(w, `{"error":"Ошибка при обновлении задачи"}`, http.StatusInternalServerError)
				log.Println("Ошибка при обновлении задачи", err)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}
}

func updateTaskHandler(database *sql.DB, w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&task)
	if err != nil {
		http.Error(w, `{"error":"Неверный формат данных"}`, http.StatusBadRequest)
		log.Println("Неверный формат данных", err)
		return
	}

	if task.ID == "0" {
		http.Error(w, `{"error":"Не указан идентификатор"}`, http.StatusBadRequest)
		return
	}

	if task.Title == "" {
		http.Error(w, `{"error":"Не указан заголовок задачи"}`, http.StatusBadRequest)
		return
	}

	var taskDate time.Time
	if task.Date == "" {
		taskDate = time.Now()
	} else {
		taskDate, err = time.Parse("20060102", task.Date)
		if err != nil {
			http.Error(w, `{"error":"Неверный формат даты"}`, http.StatusBadRequest)
			log.Println("Неверный формат даты", err)
			return
		}
	}

	now := time.Now()
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if taskDate.Format("20060102") < now.Format("20060102") {
		if task.Repeat == "" {
			http.Error(w, `{"error":"Дата не может быть в прошлом"}`, http.StatusBadRequest)
			return
		} else {
			taskDateStr := taskDate.Format("20060102")
			nextDate, err := NextDate(now, taskDateStr, task.Repeat)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
				log.Println("Ошибка при рассчете даты", err)
				return
			}
			taskDate, err = time.Parse("20060102", nextDate)
			if err != nil {
				http.Error(w, `{"error":"Ошибка при вычислении следующей даты"}`, http.StatusInternalServerError)
				log.Println("Ошибка при рассчете даты", err)
				return
			}
		}
	}

	var existingID int
	err = database.QueryRow("SELECT id FROM scheduler WHERE id = ?", task.ID).Scan(&existingID)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
		log.Println("Задача не найдена", err)
		return
	} else if err != nil {
		http.Error(w, `{"error":"Ошибка при проверке задачи"}`, http.StatusInternalServerError)
		log.Println("Ошибка при проверке задачи", err)
		return
	}

	query := `UPDATE scheduler SET date = ?, title = ?, comment = ?, repeat = ? WHERE id = ?`
	_, err = database.Exec(query, taskDate.Format("20060102"), task.Title, task.Comment, task.Repeat, task.ID)
	if err != nil {
		http.Error(w, `{"error":"Ошибка при обновлении задачи"}`, http.StatusInternalServerError)
		log.Println("Ошибка при обновлении задачи", err)
		return
	}

	response := map[string]interface{}{
		"id":      task.ID,
		"date":    taskDate.Format(dateFormat),
		"title":   task.Title,
		"comment": task.Comment,
		"repeat":  task.Repeat,
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func getTaskHandler(database *sql.DB, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	idParam := r.URL.Query().Get("id")
	fmt.Println("Полученный ID:", idParam)

	if idParam == "" {
		fmt.Println("Ошибка: ID не указан в запросе")
		http.Error(w, `{"error":"Не указан идентификатор"}`, http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idParam)
	if err != nil || id <= 0 {
		http.Error(w, `{"error":"Указан некорректный идентификатор"}`, http.StatusBadRequest)
		log.Println("Указан некорректный идентификатор", err)
		return
	}

	query := `SELECT id, date, title, comment, repeat FROM scheduler WHERE id = ?`

	err = database.QueryRow(query, id).Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"Задача не найдена"}`, http.StatusNotFound)
		log.Println("Задача не найдена", err)
		return
	} else if err != nil {
		log.Println("Ошибка при выполнении SQL-запроса:", err)
		http.Error(w, `{"error":"Ошибка при извлечении задачи из базы данных"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(task)
	if err != nil {
		http.Error(w, `{"error":"Ошибка при формировании ответа"}`, http.StatusInternalServerError)
		log.Println("Ошибка при формировании ответа", err)
	}
}

func GetTasksHandler(database *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		query := `SELECT id, date, title, comment, repeat FROM scheduler WHERE date >= ? ORDER BY date LIMIT 50`
		now := time.Now().Format("20060102")
		rows, err := database.Query(query, now)
		if err != nil {
			http.Error(w, `{"error":"Ошибка при извлечении задач из базы данных"}`, http.StatusInternalServerError)
			log.Println("Ошибка базы данных", err)
			return
		}

		var tasks []map[string]string
		for rows.Next() {
			var id int
			var date, title, comment, repeat string
			err := rows.Scan(&id, &date, &title, &comment, &repeat)
			if err != nil {
				http.Error(w, `{"error":"Ошибка при чтении данных задачи"}`, http.StatusInternalServerError)
				log.Println("Ошибка при чтении данных задачи", err)
				return
			}

			task := map[string]string{
				"id":      strconv.Itoa(id),
				"date":    date,
				"title":   title,
				"comment": comment,
				"repeat":  repeat,
			}
			tasks = append(tasks, task)
		}

		if len(tasks) == 0 {
			tasks = []map[string]string{}
		}

		response := map[string]interface{}{
			"tasks": tasks,
		}

		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			http.Error(w, `{"error":"Ошибка при формировании ответа"}`, http.StatusInternalServerError)
			log.Println("Ошибка при формировании ответа", err)
		}
	}
}

func createTaskHandler(database *sql.DB, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	var newTask struct {
		Date    string `json:"date"`
		Title   string `json:"title"`
		Comment string `json:"comment"`
		Repeat  string `json:"repeat"`
	}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&newTask)
	if err != nil {
		http.Error(w, `{"error":"Неверный формат данных"}`, http.StatusBadRequest)
		log.Println("Неверный формат данных", err)
		return
	}

	if newTask.Title == "" {
		http.Error(w, `{"error":"Не указан заголовок задачи"}`, http.StatusBadRequest)
		return
	}

	var taskDate time.Time
	if newTask.Date == "" {
		taskDate = time.Now()
	} else {
		taskDate, err = time.Parse(dateFormat, newTask.Date)
		if err != nil {
			http.Error(w, `{"error":"Неверный формат даты"}`, http.StatusBadRequest)
			log.Println("Неверный формат даты", err)
			return
		}
	}

	now := time.Now()
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if taskDate.Format(dateFormat) < now.Format(dateFormat) {
		if newTask.Repeat == "" {
			taskDate = now
		} else {
			nextDate, err := NextDate(now, taskDate.Format(dateFormat), newTask.Repeat)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
				log.Println("Ошибка при рассчете даты", err)
				return
			}
			taskDate, err = time.Parse(dateFormat, nextDate)
			if err != nil {
				http.Error(w, `{"error":"Ошибка при вычислении следующей даты"}`, http.StatusInternalServerError)
				log.Println("Ошибка при рассчете даты", err)
				return
			}
		}
	} else if taskDate.Format(dateFormat) == now.Format(dateFormat) {
	}

	query := `INSERT INTO scheduler (date, title, comment, repeat) VALUES (?, ?, ?, ?)`
	res, err := database.Exec(query, taskDate.Format(dateFormat), newTask.Title, newTask.Comment, newTask.Repeat)
	if err != nil {
		http.Error(w, `{"error":"Ошибка при добавлении задачи в базу данных"}`, http.StatusInternalServerError)
		log.Println("Ошибка базы данных", err)
		return
	}

	id, err := res.LastInsertId()
	if err != nil {
		http.Error(w, `{"error":"Не удалось получить id задачи"}`, http.StatusInternalServerError)
		log.Println("Не удалось получить id задачи", err)
		return
	}

	response := map[string]interface{}{
		"id":      id,
		"date":    taskDate.Format(dateFormat),
		"title":   newTask.Title,
		"comment": newTask.Comment,
		"repeat":  newTask.Repeat,
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func NextDateHandler(w http.ResponseWriter, r *http.Request) {
	nowStr := r.URL.Query().Get("now")
	dateStr := r.URL.Query().Get("date")
	repeat := r.URL.Query().Get("repeat")

	if nowStr == "" || dateStr == "" || repeat == "" {
		http.Error(w, "Все параметры (now, date, repeat) обязательны", http.StatusBadRequest)
		return
	}

	now, err := time.Parse("20060102", nowStr)
	if err != nil {
		http.Error(w, "Неверный формат даты now", http.StatusBadRequest)
		log.Println("Неверный формат даты now", err)
		return
	}

	nextDate, err := NextDate(now, dateStr, repeat)
	if err != nil {
		http.Error(w, fmt.Sprintf("Ошибка при вычислении следующей даты: %v", err), http.StatusBadRequest)
		log.Println("Ошибка при рассчете даты", err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, nextDate)
}

func NextDate(now time.Time, date string, repeat string) (string, error) {
	now = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	taskDate, err := time.Parse(dateFormat, date)
	if err != nil {
		log.Println("Неверный формат даты", err)
		return "", fmt.Errorf("неверный формат даты")
	}

	parts := strings.Fields(repeat)
	if len(parts) == 0 {
		log.Println("Пустое правило повторения", err)
		return "", fmt.Errorf("пустое правило повторения")
	}

	switch parts[0] {
	case "d":
		if len(parts) != 2 {
			return "", fmt.Errorf("неверный формат repeat")
		}
		days, err := strconv.Atoi(parts[1])
		if err != nil || days <= 0 || days > 400 {
			log.Println("Неверное соблюдение правил", err)
			return "", fmt.Errorf("неверное количество дней в repeat")
		}

		taskDate = taskDate.AddDate(0, 0, days)

		for taskDate.Format(dateFormat) <= now.Format(dateFormat) {
			taskDate = taskDate.AddDate(0, 0, days)
		}

		return taskDate.Format(dateFormat), nil

	case "y":
		taskDate = taskDate.AddDate(1, 0, 0)

		for taskDate.Format(dateFormat) <= now.Format(dateFormat) {
			taskDate = taskDate.AddDate(1, 0, 0)
		}

		return taskDate.Format(dateFormat), nil

	default:
		return "", fmt.Errorf("неизвестный формат repeat")
	}
}
