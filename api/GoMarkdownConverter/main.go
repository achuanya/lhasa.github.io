package main

import (
	"io/ioutil"
	"os"

	"github.com/russross/blackfriday/v2"
)

func main() {
	input, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	output := blackfriday.Run(input)

	os.Stdout.Write(output)
}
