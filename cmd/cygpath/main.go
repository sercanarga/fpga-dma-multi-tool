package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	arguments := os.Args[1:]
	if len(arguments) > 0 && (arguments[0] == "-m" || arguments[0] == "-u") {
		arguments = arguments[1:]
	}
	path := strings.Join(arguments, " ")
	path = strings.ReplaceAll(path, `\`, "/")
	fmt.Println(path)
}
