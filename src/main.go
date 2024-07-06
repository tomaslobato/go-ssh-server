package main

import (
	"fmt"
	"log"
	"net"
	"os"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/ssh"
)

func main() {
	db, err := initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	//setup ssh
	config := &ssh.ServerConfig{
		PasswordCallback: passwordCallback,
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

	//tcp server on port 2222
	listener, err := net.Listen("tcp", "0.0.0.0:2222")
	if err != nil {
		log.Fatal("failed to listen for connection: ", err)
	}
	//log IP for easy connection
	fmt.Printf("listening on %s:2222\n", getLocalIP())

	//get connections
	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Fatal("failed accepting connection:", err)
		}

		go handleConnection(db, nConn, config)
	}
}
