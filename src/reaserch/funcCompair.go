package main

import (
	"fmt"
	"reflect"
)

func SomeFun()    {}
func AnotherFun() {}

func main() {
	sf1 := reflect.ValueOf(SomeFun)
	sf2 := reflect.ValueOf(SomeFun)
	fmt.Println(sf1.Pointer() == sf2.Pointer()) // Prints true

	af1 := reflect.ValueOf(AnotherFun)
	fmt.Println(sf1.Pointer() == af1.Pointer()) // Prints false
}
