package main

import (
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

	matrix, reviewsRaw := extract()
	games, prices, dates, platforms, priceLogs, reviews := transform(matrix, reviewsRaw)
	load(games, prices, dates, platforms, priceLogs, reviews)
}

func processArgs(args []string) {
	for i := 1; i < len(args); i += 2 {
		if i+1 >= len(args) {
			panic("Missing argument value!")
		}

		flag := args[i]
		value := strings.TrimSpace(args[i+1])

		switch flag {
		case "-m":
			switch value {
			case "full":
				ctx.full = true
			case "snapshot":
				ctx.full = false
			default:
				panic("Invalid mode argument!")
			}

		case "-k":
			apiKeyITAD = value

		default:
			panic("Invalid parameter: " + flag)
		}
	}
}
