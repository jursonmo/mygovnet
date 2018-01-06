package vnet

import (
	"crypto"
	"log"
	"sync"
)

const (
	// 支持的加密方法
	//CRY_AES      = byte(0x0) // default aes-256
	CRY_XOR      = byte(0x1)
	CRY_NONE     = byte(0x2)
	CRY_AES128   = byte(0x3)
	CRY_AES192   = byte(0x4)
	CRY_AES256   = byte(0x5)
	CRY_BLOWFISH = byte(0x6)
	CRY_TWOFISH  = byte(0x7)
	CRY_CAST5    = byte(0x8)
	CRY_3DES     = byte(0x9)
	CRY_XTEA     = byte(0xA)
	CRY_SALSA20  = byte(0xB)
	CRY_END      = byte(0xC)
)

var crypts = make(map[byte]crypto.BlockCrypt)
var CRYtoString [CRY_END]string
var cryptLock sync.Mutex
var CryptType byte //= CRY_AES256

func init() {
	//CRYtoString[CRY_AES] = "aes"
	CRYtoString[CRY_XOR] = "xor"
	CRYtoString[CRY_NONE] = "none"
	CRYtoString[CRY_AES128] = "aes-128"
	CRYtoString[CRY_AES192] = "aes-192"
	CRYtoString[CRY_AES256] = "aes"
	CRYtoString[CRY_BLOWFISH] = "blowfish"
	CRYtoString[CRY_TWOFISH] = "twofish"
	CRYtoString[CRY_CAST5] = "cast5"
	CRYtoString[CRY_3DES] = "3des"
	CRYtoString[CRY_XTEA] = "xtea"
	CRYtoString[CRY_SALSA20] = "salsa20"

	for c := byte(1); c < CRY_END; c++ {
		crypts[c], _ = crypto.NewBlockCrypt(CRYtoString[c])
	}
	//log.Printf("crypto init ok\n")
}

func SetDefaultCryptType(cts string) {
	for ct, s := range CRYtoString {
		if s == cts {
			CryptType = byte(ct)
			log.Printf("=========set default crypto type ok, cts=%s, ct=%d==========\n", cts, ct)
			return
		}
	}
	log.Panicf("no support %s, just support %v\n", cts, CRYtoString)
}
