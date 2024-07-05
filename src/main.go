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
		"tomas": "opusk",
	}

	if pass, ok := users[username]; ok {
		return pass, nil
	}
	return "", fmt.Errorf("user not found")
}
func main() {
	db, err := initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

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

	privateBytes, err := os.ReadFile("/root/.ssh/id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key: ", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key: ", err)
	}

	config.AddHostKey(private)

	listener, err := net.Listen("tcp", "0.0.0.0:2222")
	if err != nil {
		log.Fatal("failed to listen for connection: ", err)
	}
	fmt.Printf("listening on %s:2222\n", getLocalIP())

	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Fatal("failed accepting connection:", err)
		}

		go handleConnection(db, nConn, config)
	}
}

func handleConnection(db *sql.DB, nConn net.Conn, config *ssh.ServerConfig) {
	if db == nil {
		log.Println("Database connection is nil")
		return
	}

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

		io.WriteString(channel, "Welcome to the Go SSH server!\r\n> ")

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

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "Error getting IP"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "No IP found"
}
