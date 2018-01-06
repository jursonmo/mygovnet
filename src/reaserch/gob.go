package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

type Person struct {
	Name string
	age  int
}

func main() {
	p := Person{"mjw", 29}
	buf := bytes.NewBuffer(nil)
	fmt.Println(len(buf.Bytes()))
	en := gob.NewEncoder(buf)
	en.Encode(p)
	fmt.Println(buf.Bytes())
}
