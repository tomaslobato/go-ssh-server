package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

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

	privateBytes, err := os.ReadFile("/home/tomas/.ssh/id_rsa") ///root/.ssh/id_rsa
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

		go handleConnection(nConn, config)
	}
}

func handleConnection(nConn net.Conn, config *ssh.ServerConfig) {
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

				// Echo the input back to the client
				channel.Write(buf)

				if buf[0] == '\r' {
					channel.Write([]byte{'\n'})
					trimmedInput := strings.TrimSpace(string(command))
					handleCommand(trimmedInput, channel)
					command = nil // Clear the buffer for the next input
				} else {
					command = append(command, buf[0]) //append char to the command
				}
			}
		}()
	}
}

func handleCommand(command string, channel ssh.Channel) {
	switch command {
	case "opusk":
		channel.Write([]byte("ksupo\r\n"))
	case "exit":
		channel.Write([]byte("bye!\r\n"))
		channel.Close()

	}
}
