package main

import (
	"fmt"
	"os"

	"todos/internal/cli"
)

func main() {
	if err := cli.Run("tod", os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
