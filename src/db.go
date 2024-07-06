package main

import (
	"database/sql"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

func initDB() (*sql.DB, error) {
	//connecting to DB
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, dbname)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS saved (
		id SERIAL PRIMARY KEY,
		text TEXT NOT NULL
	)`)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func showSavedContent(db *sql.DB, channel ssh.Channel) {
	//get saves from saved
	rows, err := db.Query("SELECT id, text FROM saved ORDER BY id")
	if err != nil {
		channel.Write([]byte(fmt.Sprintf("Error fetching saved: %v\r\n", err)))
		return
	}
	defer rows.Close()

	channel.Write([]byte("Saved:\r\n"))
	//print all from "saved"
	for rows.Next() {
		var id int
		var text string
		if err := rows.Scan(&id, &text); err != nil {
			channel.Write([]byte(fmt.Sprintf("Error reading row: %v\r\n", err)))
			return
		}
		channel.Write([]byte(fmt.Sprintf("%d: %s\r\n", id, text)))
	}

	if err := rows.Err(); err != nil {
		channel.Write([]byte(fmt.Sprintf("Error after fetching rows: %v\r\n", err)))
	}
}
