// Command hashpw generates a bcrypt hash of the given password and prints it.
//
//	go run ./cmd/hashpw <password>
package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: hashpw <password>")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(os.Args[1]), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}

	fmt.Println(string(hash))
}
