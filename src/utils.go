package main

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
)

func getUserPassword(username string) (string, error) {
	pwd := os.Getenv("SERVER_PASSWORD")

	users := map[string]string{
		"tomas": pwd,
	}

	if pass, ok := users[username]; ok {
		return pass, nil
	}

	return "", fmt.Errorf("user not found")
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

func passwordCallback(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
	username := c.User()
	uPass, err := getUserPassword(username)
	if err != nil {
		fmt.Printf("User %s not found\n", username)
		return nil, fmt.Errorf("password rejected for %q", username)
	}

	// Compare the hashed passwords
	if string(pass) != uPass {
		fmt.Printf("Incorrect password for user %s\n", username)
		return nil, fmt.Errorf("password rejected for %q", username)
	}

	return &ssh.Permissions{}, nil
}
