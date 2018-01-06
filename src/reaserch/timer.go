package main

import (
	"fmt"
	"time"
)

func aa() (int, error) {
	return 1, nil
}
func main() {
	var err error
	a, err := aa()
	if err != nil {
		return
	}
	b, err := aa()
	if err != nil {
		return
	}
	fmt.Println(a, b)
	f := func() {
		fmt.Println("Time out")
	}
	tf := time.AfterFunc(time.Second, f)

	time.Sleep(time.Second * 2)
	tf.Stop()

	tf = myAfterFunc(time.Second, printArg, a, b)
	//tf.Stop()
	a = 9
	tf = myAfterFunc(time.Second, printArg, a, b)
	time.Sleep(time.Second * 2)
	hbTimer := time.NewTimer(time.Second * 1)
	for {
		<-hbTimer.C
		fmt.Println("sfsdfsadf")
		hbTimer.Reset(time.Second) //must need
	}

}

func printArg(arg ...interface{}) {
	fmt.Printf("arg=%v\n", arg)
}

func myAfterFunc(d time.Duration, userFunc func(...interface{}), arg ...interface{}) *time.Timer {
	f := func() {
		userFunc(arg...)
	}
	return time.AfterFunc(time.Second, f)
}
