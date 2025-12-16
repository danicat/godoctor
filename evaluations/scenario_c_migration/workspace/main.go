package main

import (
	"fmt"
	"io/ioutil" // Deprecated
	"log"
)

func main() {
	content, err := ioutil.ReadFile("data.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Read %d bytes\n", len(content))
	
	files, err := ioutil.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		fmt.Println(f.Name())
	}
}

