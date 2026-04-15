package main

import (
	"fmt"
	"os"

	scmctlapp "github.com/alexisjcarr/scm/internal/scmctl/app"
)

func main() {
	if err := scmctlapp.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
