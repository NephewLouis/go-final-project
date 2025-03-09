package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"go_final_project/config"
	"go_final_project/db"
	"go_final_project/handlers"
)

func main() {
	webDir := "web"

	port := os.Getenv("TODO_PORT")
	if port == "" {
		port = strconv.Itoa(config.Port)
	}
	port = ":" + port

	database, err := db.InitDB()
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	fmt.Println("Сервер запущен на порту", port)

	http.HandleFunc("/api/task/done", handlers.MarkTaskDoneHandler(database))

	http.HandleFunc("/api/task", handlers.TaskHandler(database))

	http.HandleFunc("/api/tasks", handlers.GetTasksHandler(database))

	http.HandleFunc("/api/nextdate", handlers.NextDateHandler)

	http.Handle("/", http.FileServer(http.Dir(webDir)))

	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal("Ошибка при запуске сервера: ", err)
	}
}
