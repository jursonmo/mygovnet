package main

import (
	"fmt"
	"mycrypto"
)

func main() {
	aes := mycrypto.AesNewEncrypt()
	err := aes.SetEncrypKey("12345678901234567")
	if err != nil {
		fmt.Println(err)
		return
	}
	enbuf, err := aes.Encrypt("mojianwei")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(enbuf), string(enbuf))

	deString, err := aes.Decrypt(enbuf)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(deString)
}
