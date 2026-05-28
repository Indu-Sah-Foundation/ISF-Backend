// Command hashpw prints a bcrypt hash for the password given on stdin or argv.
// Used for rotating ADMIN_PASSWORD: rotate the Key Vault secret, then run
// this to produce a hash and `UPDATE users SET password_hash=...` in Postgres.
// Cost factor matches auth.Service.EnsureAdmin (12) so the hash is identical
// to what the server would produce on first boot.
//
// Usage:
//   go run ./cmd/hashpw 'my-new-password'
//   echo 'my-new-password' | go run ./cmd/hashpw
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const cost = 12

func main() {
	var pw string
	if len(os.Args) > 1 {
		pw = os.Args[1]
	} else {
		r := bufio.NewReader(os.Stdin)
		line, _ := r.ReadString('\n')
		pw = strings.TrimRight(line, "\r\n")
	}
	if pw == "" {
		fmt.Fprintln(os.Stderr, "usage: hashpw <password>   (or pipe via stdin)")
		os.Exit(2)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), cost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hash failed:", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}
