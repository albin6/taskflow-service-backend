package main

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := "admin123"
	bytes, _ := bcrypt.GenerateFromPassword([]byte(password), 12)
	fmt.Println(string(bytes))
}
