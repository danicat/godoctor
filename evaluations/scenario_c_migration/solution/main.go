package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	content, err := os.ReadFile("data.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Read %d bytes\n", len(content))
	
	files, err := os.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		fmt.Println(f.Name())
	}
}

