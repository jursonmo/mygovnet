// From kcp-go

package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/sha1"
	"flag"

	"golang.org/x/crypto/blowfish"
	"golang.org/x/crypto/cast5"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/salsa20"
	"golang.org/x/crypto/tea"
	"golang.org/x/crypto/twofish"
	"golang.org/x/crypto/xtea"
)

var (
	flgKey        = flag.String("key", "go_-vnet_Key", "pre-shared secret between client and server")
	initialVector = []byte{167, 115, 79, 156, 18, 172, 27, 1, 164, 21, 242, 193, 252, 120, 230, 107}
	saltxor       = `Mjw13VoF#rWLtJo6`
)

const (
	// SALT is used for pbkdf2 key expansion
	SALT = "mjw_go_-vnet"
)

// BlockCrypt defines encryption/decryption methods for a given byte slice.
// Notes on implementing: the data to be encrypted contains a builtin
// nonce at the first 16 bytes
type BlockCrypt interface {
	// Encrypt encrypts the whole block in src into dst.
	// Dst and src may point at the same memory.
	Encrypt(dst, src []byte)

	// Decrypt decrypts the whole block in src into dst.
	// Dst and src may point at the same memory.
	Decrypt(dst, src []byte)
}

type salsa20BlockCrypt struct {
	key [32]byte
}

func NewBlockCrypt(cry string) (BlockCrypt, error) {
	pass := pbkdf2.Key([]byte(*flgKey), []byte(SALT), 4096, 32, sha1.New)
	switch cry {
	case "tea":
		return NewTEABlockCrypt(pass[:16])
	case "xor":
		return NewSimpleXORBlockCrypt(pass)
	case "none":
		return NewNoneBlockCrypt(pass)
	case "aes-128":
		return NewAESBlockCrypt(pass[:16])
	case "aes-192":
		return NewAESBlockCrypt(pass[:24])
	case "blowfish":
		return NewBlowfishBlockCrypt(pass)
	case "twofish":
		return NewTwofishBlockCrypt(pass)
	case "cast5":
		return NewCast5BlockCrypt(pass[:16])
	case "3des":
		return NewTripleDESBlockCrypt(pass[:24])
	case "xtea":
		return NewXTEABlockCrypt(pass[:16])
	case "salsa20":
		return NewSalsa20BlockCrypt(pass)
	default:
		// aes: aes-256
		return NewAESBlockCrypt(pass)
	}
}

// NewSalsa20BlockCrypt https://en.wikipedia.org/wiki/Salsa20
func NewSalsa20BlockCrypt(key []byte) (BlockCrypt, error) {
	c := new(salsa20BlockCrypt)
	copy(c.key[:], key)
	return c, nil
}

func (c *salsa20BlockCrypt) Encrypt(dst, src []byte) {
	salsa20.XORKeyStream(dst[8:], src[8:], src[:8], &c.key)
	copy(dst[:8], src[:8])
}
func (c *salsa20BlockCrypt) Decrypt(dst, src []byte) {
	salsa20.XORKeyStream(dst[8:], src[8:], src[:8], &c.key)
	copy(dst[:8], src[:8])
}

type twofishBlockCrypt struct {
	encbuf []byte
	decbuf []byte
	block  cipher.Block
}

// NewTwofishBlockCrypt https://en.wikipedia.org/wiki/Twofish
func NewTwofishBlockCrypt(key []byte) (BlockCrypt, error) {
	c := new(twofishBlockCrypt)
	block, err := twofish.NewCipher(key)
	if err != nil {
		return nil, err
	}
	c.block = block
	c.encbuf = make([]byte, twofish.BlockSize)
	c.decbuf = make([]byte, 2*twofish.BlockSize)
	return c, nil
}

func (c *twofishBlockCrypt) Encrypt(dst, src []byte) { encrypt(c.block, dst, src, c.encbuf) }
func (c *twofishBlockCrypt) Decrypt(dst, src []byte) { decrypt(c.block, dst, src, c.decbuf) }

type tripleDESBlockCrypt struct {
	encbuf []byte
	decbuf []byte
	block  cipher.Block
}

// NewTripleDESBlockCrypt https://en.wikipedia.org/wiki/Triple_DES
func NewTripleDESBlockCrypt(key []byte) (BlockCrypt, error) {
	c := new(tripleDESBlockCrypt)
	block, err := des.NewTripleDESCipher(key)
	if err != nil {
		return nil, err
	}
	c.block = block
	c.encbuf = make([]byte, des.BlockSize)
	c.decbuf = make([]byte, 2*des.BlockSize)
	return c, nil
}

func (c *tripleDESBlockCrypt) Encrypt(dst, src []byte) { encrypt(c.block, dst, src, c.encbuf) }
func (c *tripleDESBlockCrypt) Decrypt(dst, src []byte) { decrypt(c.block, dst, src, c.decbuf) }

type cast5BlockCrypt struct {
	encbuf []byte
	decbuf []byte
	block  cipher.Block
}

// NewCast5BlockCrypt https://en.wikipedia.org/wiki/CAST-128
func NewCast5BlockCrypt(key []byte) (BlockCrypt, error) {
	c := new(cast5BlockCrypt)
	block, err := cast5.NewCipher(key)
	if err != nil {
		return nil, err
	}
	c.block = block
	c.encbuf = make([]byte, cast5.BlockSize)
	c.decbuf = make([]byte, 2*cast5.BlockSize)
	return c, nil
}

func (c *cast5BlockCrypt) Encrypt(dst, src []byte) { encrypt(c.block, dst, src, c.encbuf) }
func (c *cast5BlockCrypt) Decrypt(dst, src []byte) { decrypt(c.block, dst, src, c.decbuf) }

type blowfishBlockCrypt struct {
	encbuf []byte
	decbuf []byte
	block  cipher.Block
}

// NewBlowfishBlockCrypt https://en.wikipedia.org/wiki/Blowfish_(cipher)
func NewBlowfishBlockCrypt(key []byte) (BlockCrypt, error) {
	c := new(blowfishBlockCrypt)
	block, err := blowfish.NewCipher(key)
	if err != nil {
		return nil, err
	}
	c.block = block
	c.encbuf = make([]byte, blowfish.BlockSize)
	c.decbuf = make([]byte, 2*blowfish.BlockSize)
	return c, nil
}

func (c *blowfishBlockCrypt) Encrypt(dst, src []byte) { encrypt(c.block, dst, src, c.encbuf) }
func (c *blowfishBlockCrypt) Decrypt(dst, src []byte) { decrypt(c.block, dst, src, c.decbuf) }

type aesBlockCrypt struct {
	encbuf []byte
	decbuf []byte
	block  cipher.Block
}

// NewAESBlockCrypt https://en.wikipedia.org/wiki/Advanced_Encryption_Standard
func NewAESBlockCrypt(key []byte) (BlockCrypt, error) {
	c := new(aesBlockCrypt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	c.block = block
	c.encbuf = make([]byte, aes.BlockSize)
	c.decbuf = make([]byte, 2*aes.BlockSize)
	return c, nil
}

func (c *aesBlockCrypt) Encrypt(dst, src []byte) { encrypt(c.block, dst, src, c.encbuf) }
func (c *aesBlockCrypt) Decrypt(dst, src []byte) { decrypt(c.block, dst, src, c.decbuf) }

type teaBlockCrypt struct {
	encbuf []byte
	decbuf []byte
	block  cipher.Block
}

// NewTEABlockCrypt https://en.wikipedia.org/wiki/Tiny_Encryption_Algorithm
func NewTEABlockCrypt(key []byte) (BlockCrypt, error) {
	c := new(teaBlockCrypt)
	block, err := tea.NewCipherWithRounds(key, 16)
	if err != nil {
		return nil, err
	}
	c.block = block
	c.encbuf = make([]byte, tea.BlockSize)
	c.decbuf = make([]byte, 2*tea.BlockSize)
	return c, nil
}

func (c *teaBlockCrypt) Encrypt(dst, src []byte) { encrypt(c.block, dst, src, c.encbuf) }
func (c *teaBlockCrypt) Decrypt(dst, src []byte) { decrypt(c.block, dst, src, c.decbuf) }

type xteaBlockCrypt struct {
	encbuf []byte
	decbuf []byte
	block  cipher.Block
}

// NewXTEABlockCrypt https://en.wikipedia.org/wiki/XTEA
func NewXTEABlockCrypt(key []byte) (BlockCrypt, error) {
	c := new(xteaBlockCrypt)
	block, err := xtea.NewCipher(key)
	if err != nil {
		return nil, err
	}
	c.block = block
	c.encbuf = make([]byte, xtea.BlockSize)
	c.decbuf = make([]byte, 2*xtea.BlockSize)
	return c, nil
}

func (c *xteaBlockCrypt) Encrypt(dst, src []byte) { encrypt(c.block, dst, src, c.encbuf) }
func (c *xteaBlockCrypt) Decrypt(dst, src []byte) { decrypt(c.block, dst, src, c.decbuf) }

type simpleXORBlockCrypt struct {
	xortbl []byte
}

// NewSimpleXORBlockCrypt simple xor with key expanding
func NewSimpleXORBlockCrypt(key []byte) (BlockCrypt, error) {
	c := new(simpleXORBlockCrypt)
	c.xortbl = pbkdf2.Key(key, []byte(saltxor), 32, 1500, sha1.New)
	return c, nil
}

func (c *simpleXORBlockCrypt) Encrypt(dst, src []byte) { xorBytes(dst, src, c.xortbl) }
func (c *simpleXORBlockCrypt) Decrypt(dst, src []byte) { xorBytes(dst, src, c.xortbl) }

type noneBlockCrypt struct{}

// NewNoneBlockCrypt does nothing but copying
func NewNoneBlockCrypt(key []byte) (BlockCrypt, error) {
	return new(noneBlockCrypt), nil
}

func (c *noneBlockCrypt) Encrypt(dst, src []byte) { copy(dst, src) }
func (c *noneBlockCrypt) Decrypt(dst, src []byte) { copy(dst, src) }

// packet encryption with local CFB mode
func encrypt(block cipher.Block, dst, src, buf []byte) {
	blocksize := block.BlockSize()
	tbl := buf[:blocksize]
	block.Encrypt(tbl, initialVector)
	n := len(src) / blocksize
	base := 0
	for i := 0; i < n; i++ {
		xorWords(dst[base:], src[base:], tbl)
		block.Encrypt(tbl, dst[base:])
		base += blocksize
	}
	xorBytes(dst[base:], src[base:], tbl)
}

func decrypt(block cipher.Block, dst, src, buf []byte) {
	blocksize := block.BlockSize()
	tbl := buf[:blocksize]
	next := buf[blocksize:]
	block.Encrypt(tbl, initialVector)
	n := len(src) / blocksize
	base := 0
	for i := 0; i < n; i++ {
		block.Encrypt(next, src[base:])
		xorWords(dst[base:], src[base:], tbl)
		tbl, next = next, tbl
		base += blocksize
	}
	xorBytes(dst[base:], src[base:], tbl)
}
