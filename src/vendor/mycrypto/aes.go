package mycrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"math/rand"
	"time"
)

type AesEncrypt struct {
	key []byte
}

func AesNewEncrypt() *AesEncrypt {
	return &AesEncrypt{}
}

func (ae *AesEncrypt) SetEncrypKey(passwd string) error {
	var err error
	ae.key, err = ae.genKey(passwd)
	return err
}

func (ae *AesEncrypt) genKey(passwd string) ([]byte, error) {
	//strKey := "1234567890123456"
	keyLen := len(passwd)
	if keyLen < 16 {
		return nil, errors.New("key less then 16")
	}
	arrKey := []byte(passwd)
	if keyLen >= 32 {
		//取前32个字节
		return arrKey[:32], nil
	}
	if keyLen >= 24 {
		//取前24个字节
		return arrKey[:24], nil
	}
	//取前16个字节
	return arrKey[:16], nil
}

//加密字符串
func (this *AesEncrypt) Encrypt(strMesg string) ([]byte, error) {
	key := this.key
	var iv = []byte(key)[:aes.BlockSize]
	encrypted := make([]byte, len(strMesg))
	aesBlockEncrypter, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesEncrypter := cipher.NewCFBEncrypter(aesBlockEncrypter, iv) //模式有ECB，CBC，OFB，CFB，CTR和XTS等
	aesEncrypter.XORKeyStream(encrypted, []byte(strMesg))
	return encrypted, nil
}

//解密字符串
func (this *AesEncrypt) Decrypt(src []byte) (Desc []byte, err error) {
	defer func() {
		//错误处理
		if e := recover(); e != nil {
			err = e.(error)
		}
	}()
	key := this.key
	var iv = []byte(key)[:aes.BlockSize]
	decrypted := make([]byte, len(src))
	var aesBlockDecrypter cipher.Block
	aesBlockDecrypter, err = aes.NewCipher([]byte(key))
	if err != nil {
		return
	}
	aesDecrypter := cipher.NewCFBDecrypter(aesBlockDecrypter, iv)
	aesDecrypter.XORKeyStream(decrypted, src)
	return decrypted, nil
}

// func main() {
// 	fmt.Println(RandStringBytes(32))
// }

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
