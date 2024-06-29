package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"database/sql"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/ssh"
)

func getUserPassword(username string) (string, error) {

	// This is just an example. Don't store passwords like this in production!
	users := map[string]string{
		"tomas": "ironman sucks",
	}

	if pass, ok := users[username]; ok {
		return pass, nil
	}
	return "", fmt.Errorf("user not found")
}

func main() {
	db, err := initDB()
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			username := c.User()
			uPass, err := getUserPassword(username)
			if err != nil {
				fmt.Printf("User %s not found\n", username)
				return nil, fmt.Errorf("password rejected for %q", username)
			}

			if string(pass) != uPass {
				fmt.Printf("Incorrect password for user %s\n", username)
				return nil, fmt.Errorf("password rejected for %q", username)
			}

			fmt.Printf("Correct!")
			return &ssh.Permissions{}, nil
		},
	}

	privateBytes, err := os.ReadFile("/root/.ssh/id_rsa") ///home/tomas/.ssh/id_rsa
	if err != nil {
		log.Fatal("Failed to load private key: ", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key: ", err)
	}

	config.AddHostKey(private)

	//listen port 2222
	listener, err := net.Listen("tcp", "0.0.0.0:2222")
	if err != nil {
		log.Fatal("failed to listen for connection: ", err)
	}
	fmt.Println("listening on 0.0.0.0:2222")

	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Fatal("failed accepting connection:", err)
		}

		go handleConnection(db, nConn, config)
	}
}

func handleConnection(db *sql.DB, nConn net.Conn, config *ssh.ServerConfig) {
	fmt.Println("New connection received")
	conn, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		log.Fatal("failed to handshake:", err)
		return
	}
	defer conn.Close()

	log.Println("logged in with user", conn.User())

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Fatal("Couldn't accept channel:", err)
		}

		go func(in <-chan *ssh.Request) {
			for req := range in {
				switch req.Type {
				case "shell":
					if len(req.Payload) == 0 {
						req.Reply(true, nil)
					}
				case "pty-req":
					req.Reply(true, nil)
				}
			}
		}(requests)

		io.WriteString(channel, "Welcome to the Go SSH server!\r\n")

		go func() {
			defer channel.Close()

			var command []byte

			for {
				buf := make([]byte, 1)
				_, err := channel.Read(buf)
				if err != nil {
					if err != io.EOF {
						log.Println("Error reading from channel:", err)
					}
					break
				}

				if buf[0] == 8 || buf[0] == 127 {
					if len(command) > 0 {
						command = command[:len(command)-1] // remove last character

						channel.Write([]byte("\b \b")) //move one back, space, one back again
					}
				} else if (buf[0] >= 32 && buf[0] <= 126) || buf[0] == '\r' { //only ascii
					if buf[0] == '\r' {
						channel.Write([]byte{'\r', '\n'})
						trimmedInput := strings.TrimSpace(string(command))
						handleCommand(db, trimmedInput, channel)
						command = nil               // Clear the buffer for the next input
						channel.Write([]byte("> ")) // Print a new prompt
					} else {
						channel.Write(buf) // Echo the character to client
						command = append(command, buf[0])
					}
				}
			}
		}()
	}
}

func handleCommand(db *sql.DB, command string, channel ssh.Channel) {
	fmt.Printf("Received command: %s\n", command)

	switch {
	case strings.HasPrefix(command, "save "):
		fmt.Println("Detected 'save' command")
		parts := strings.SplitN(command, " ", 2)
		if len(parts) == 2 {
			content := parts[1]
			fmt.Printf("Content to save: %s\n", content)

			_, err := db.Exec("INSERT INTO content (text) VALUES ($1)", content)
			if err != nil {
				channel.Write([]byte(fmt.Sprintf("Failed to save content: %v\r\n", err)))
				return
			}
			channel.Write([]byte(fmt.Sprintf("Content saved: %s\r\n", content)))
		} else {
			channel.Write([]byte("Invalid save command. Usage: save <content>\r\n"))
		}

	case command == "saved":
		showSavedContent(db, channel)

	case command == "exit":
		channel.Write([]byte("bye!\r\n"))
		channel.Close()

	case command == "":
		// Do nothing for empty commands
		channel.Write([]byte("> "))

	default:
		channel.Write([]byte(fmt.Sprintf("Unknown command: %s\r\n", command)))
	}
}

func initDB() (*sql.DB, error) {
	connStr := "postgres://postgres:postgres@db:5432/postgres?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS content (
		id SERIAL PRIMARY KEY,
		text TEXT NOT NULL
	)`)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func showSavedContent(db *sql.DB, channel ssh.Channel) {
	rows, err := db.Query("SELECT id, text FROM content ORDER BY id")
	if err != nil {
		channel.Write([]byte(fmt.Sprintf("Error fetching content: %v\r\n", err)))
		return
	}
	defer rows.Close()

	channel.Write([]byte("Saved content:\r\n"))
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
