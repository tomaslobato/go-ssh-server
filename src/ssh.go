package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	"golang.org/x/crypto/ssh"
)

func handleConnection(db *sql.DB, nConn net.Conn, config *ssh.ServerConfig) {
	if db == nil {
		log.Println("Database connection is nil")
		return
	}

	//create connections
	conn, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		log.Fatal("failed to handshake:", err)
		return
	}
	defer conn.Close() // handles connection close when handleConnection finishes

	go ssh.DiscardRequests(reqs) // ignore unnecessary global requests while the server keeps running

	// handle session channels
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		//accept only session channels
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Fatal("Couldn't accept channel:", err)
		}

		//handle session channel requests while the program runs
		go func(in <-chan *ssh.Request) {
			for req := range in { //iterate incoming reqs
				switch req.Type { //depending req type reply with success
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

		//handle user commands
		go func() {
			defer channel.Close() // ensure channel closes when goroutine exits

			var command []byte //buffer

			//reading from the channel
			for {
				buf := make([]byte, 1)      //mini buffer for one byte
				_, err := channel.Read(buf) //read channel's incoming byte to buf
				if err != nil {
					if err != io.EOF {
						log.Fatal("Error reading from channel:", err)
					}
					break
				}

				//handle backspace
				if buf[0] == 8 || buf[0] == 127 {
					if len(command) > 0 {
						command = command[:len(command)-1] // remove last character

						channel.Write([]byte("\b \b")) //move one back, space, one back again
					}
				} else if (buf[0] >= 32 && buf[0] <= 126) || buf[0] == '\r' { // handle regular input (only ascii)
					if buf[0] == '\r' { //enter
						channel.Write([]byte{'\r', '\n'})
						trimmedInput := strings.TrimSpace(string(command))
						handleCommand(db, trimmedInput, channel)
						command = nil // Clear the buffer for the next input
						channel.Write([]byte("> "))
					} else { //just get the character
						channel.Write(buf) // Echo the character to client
						command = append(command, buf[0])
					}
				}
			}
		}()
	}
}

// when hit enter, handle the command
func handleCommand(db *sql.DB, command string, channel ssh.Channel) {
	log.Printf("Received command: %s\n", command)

	switch {
	case strings.HasPrefix(command, "save "):
		parts := strings.SplitN(command, " ", 2) //separate it on the " "
		if len(parts) == 2 {
			content := parts[1] //content to save is second worda

			_, err := db.Exec("INSERT INTO saves (text) VALUES ($1)", content) //save to DB
			if err != nil {
				channel.Write([]byte(fmt.Sprintf("Failed to save: %v\r\n", err)))
				return
			}
			channel.Write([]byte(fmt.Sprintf("Saved: %s\r\n", content)))
		} else {
			channel.Write([]byte("Invalid save command. Usage: save <content>\r\n"))
		}

	case command == "saved": //show saved stuff
		showSavedContent(db, channel)

	case command == "exit": //close channel
		channel.Write([]byte("bye!\r\n"))
		channel.Close()

	case command == "":
		// Do nothing for empty commands
		channel.Write([]byte("> "))

	default:
		channel.Write([]byte(fmt.Sprintf("Unknown command: %s\r\n", command)))
	}
}
