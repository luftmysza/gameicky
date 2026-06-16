package main

import (
	// "bufio"
	// "fmt"
	"os"
	"strings"
)

type context struct {
	full bool
}

var (
	apiKeyITAD string
	ctx        context
)

func main() {
	ctx = context{true}

	args := os.Args
	processArgs(args)

	matrix := extract()
	games, priceLogs := transform(matrix)
	load(games, priceLogs)
}

func processArgs(args []string) {
	if len(args)%2 != 0 {
		panic("Invalid arguments!")
	}

	for i, arg := range args {
		if i%2 == 0 {
			continue
		}

		switch arg {
		case "-m":
			if len(args) >= i+1 {
				mode := args[i+1]
				switch mode {
				case "full":
					ctx.full = true
				case "snapshot":
					ctx.full = false
				default:
					panic("Invalid mode argument!")
				}
			}
		case "-k":
			if len(args) >= i+1 {
				apiKeyITAD = strings.TrimSpace(args[i+1])
			}
		default:
			panic("Invalid parameter!")
		}
	}
}
