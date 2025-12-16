package main

import (
	"bytes"
	"io"
)

func ProcessData(r *bytes.Reader) []byte {
	b, _ := io.ReadAll(r)
	return b
}
