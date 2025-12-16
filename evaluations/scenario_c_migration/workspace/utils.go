package main

import (
	"bytes"
	"io/ioutil" // Deprecated
)

func ProcessData(r *bytes.Reader) []byte {
	b, _ := ioutil.ReadAll(r)
	return b
}
