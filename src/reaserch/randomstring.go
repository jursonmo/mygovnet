package main

import (
	"fmt"
	"math/rand"
	"time"
)

func main() {
	fmt.Println(RandStringBytes(32))
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	rand.Seed(time.Now().UnixNano()) //if miss this, result always same
	l := len(letterBytes)
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(l)]
	}
	return string(b)
}
