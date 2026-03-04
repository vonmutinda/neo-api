package main

import (
	"fmt"
	"os"

	"github.com/vonmutinda/neo/pkg/password"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: seed-admin <password>")
		os.Exit(1)
	}
	hash := password.GeneratePasswordHash(os.Args[1])
	fmt.Print(hash)
}
