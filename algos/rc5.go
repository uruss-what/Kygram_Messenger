package algos

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	nRounds = 16
)

type RC5 struct {
	keys  []uint64
	words uint
}

func NewRC5() (*RC5, error) {
	r := &RC5{
		words: 64,
		keys:  make([]uint64, 2*(nRounds+1)),
	}
	return r, nil
}

func (r *RC5) CipherKey(key []byte) error {
	if len(key) < int(r.words/8) {
		return fmt.Errorf("ключ слишком короткий: требуется минимум %d байт", r.words/8)
	}
	r.keys = r.keyExpansion(key)
	return nil
}

func (r *RC5) keyExpansion(key []byte) []uint64 {
	//fmt.Println("Расширяем ключ хули")
	var Pw, Qw uint64
	switch r.words {
	case 16:
		Pw = 0xb7e1
		Qw = 0x9e37
	case 32:
		Pw = 0xb7e15163
		Qw = 0x9e3779b9
	case 64:
		Pw = 0xb7e151628aed2a6b
		Qw = 0x9e3779b97f4a7c15
	default:
		panic("unsupported word size")
	}

	c := (len(key) + int((r.words)/8) - 1) / int(r.words/8)
	L := make([]uint64, c)
	for i := len(key) - 1; i >= 0; i-- {
		L[i/int(r.words/8)] = (L[i/int(r.words/8)] << 8) + uint64(key[i])
	}

	s := make([]uint64, 2*(nRounds+1))
	s[0] = Pw
	for i := 1; i < len(s); i++ {
		s[i] = s[i-1] + Qw
	}

	i, j := 0, 0
	A, B := uint64(0), uint64(0)
	for k := 0; k < 3*(max(2*(nRounds+1), c)); k++ {
		A = rotateLeft(s[i]+A+B, 3, r.words)
		s[i] = A
		sum := L[j] + A + B
		B = rotateLeft(sum, sum%uint64(r.words), r.words)
		L[j] = B
		i = (i + 1) % int(2*(nRounds+1))
		j = (j + 1) % int(c)
	}
	return s
}

func (rc5 *RC5) Encrypt(data []byte) ([]byte, error) {
	//log.Println("Encrypt вызывается из RC5")
	return rc5.EncryptBlock(data)
}

func (rc5 *RC5) Decrypt(data []byte) ([]byte, error) {
	return rc5.DecryptBlock(data)
}

func (rc5 *RC5) EncryptBlock(block []byte) ([]byte, error) {
	wordSizeBytes := rc5.words / 8
	if len(block) != int(wordSizeBytes*2) {
		return nil, fmt.Errorf("block size must be %d bytes", wordSizeBytes*2)
	}

	var A, B uint64
	switch rc5.words {
	case 16:
		A = uint64(binary.LittleEndian.Uint16(block[0:2]))
		B = uint64(binary.LittleEndian.Uint16(block[2:4]))
	case 32:
		A = uint64(binary.LittleEndian.Uint32(block[0:4]))
		B = uint64(binary.LittleEndian.Uint32(block[4:8]))
	case 64:
		A = binary.LittleEndian.Uint64(block[0:8])
		B = binary.LittleEndian.Uint64(block[8:16])
	default:
		return nil, errors.New("unsupported word size")
	}

	A += rc5.keys[0]
	B += rc5.keys[1]
	for i := 1; i <= nRounds; i++ {
		A = rotateLeft(A^B, B%uint64(rc5.words), uint(rc5.words)) + rc5.keys[2*i]
		B = rotateLeft(B^A, A%uint64(rc5.words), uint(rc5.words)) + rc5.keys[2*i+1]
	}

	encryptedBlock := make([]byte, wordSizeBytes*2)
	switch rc5.words {
	case 16:
		binary.LittleEndian.PutUint16(encryptedBlock[0:2], uint16(A))
		binary.LittleEndian.PutUint16(encryptedBlock[2:4], uint16(B))
	case 32:
		binary.LittleEndian.PutUint32(encryptedBlock[0:4], uint32(A))
		binary.LittleEndian.PutUint32(encryptedBlock[4:8], uint32(B))
	case 64:
		binary.LittleEndian.PutUint64(encryptedBlock[0:8], A)
		binary.LittleEndian.PutUint64(encryptedBlock[8:16], B)
	}

	return encryptedBlock, nil
}

func (rc5 *RC5) DecryptBlock(block []byte) ([]byte, error) {
	wordSizeBytes := rc5.words / 8
	if len(block) != int(wordSizeBytes*2) {
		return nil, fmt.Errorf("block size must be %d bytes", wordSizeBytes*2)
	}

	var A, B uint64
	switch rc5.words {
	case 16:
		A = uint64(binary.LittleEndian.Uint16(block[0:2]))
		B = uint64(binary.LittleEndian.Uint16(block[2:4]))
	case 32:
		A = uint64(binary.LittleEndian.Uint32(block[0:4]))
		B = uint64(binary.LittleEndian.Uint32(block[4:8]))
	case 64:
		A = binary.LittleEndian.Uint64(block[0:8])
		B = binary.LittleEndian.Uint64(block[8:16])
	default:
		return nil, errors.New("unsupported word size")
	}

	for i := nRounds; i >= 1; i-- {
		B = rotateRight(B-rc5.keys[2*i+1], A%uint64(rc5.words), rc5.words) ^ A
		A = rotateRight(A-rc5.keys[2*i], B%uint64(rc5.words), rc5.words) ^ B
	}
	B -= rc5.keys[1]
	A -= rc5.keys[0]

	decryptedBlock := make([]byte, wordSizeBytes*2)
	switch rc5.words {
	case 16:
		binary.LittleEndian.PutUint16(decryptedBlock[0:2], uint16(A))
		binary.LittleEndian.PutUint16(decryptedBlock[2:4], uint16(B))
	case 32:
		binary.LittleEndian.PutUint32(decryptedBlock[0:4], uint32(A))
		binary.LittleEndian.PutUint32(decryptedBlock[4:8], uint32(B))
	case 64:
		binary.LittleEndian.PutUint64(decryptedBlock[0:8], A)
		binary.LittleEndian.PutUint64(decryptedBlock[8:16], B)
	}

	return decryptedBlock, nil
}

func rotateLeft(x, y uint64, w uint) uint64 {
	return ((x << (y % uint64(w))) | (x >> (uint64(w) - (y % uint64(w))))) & ((1 << w) - 1)
}

func rotateRight(x, y uint64, w uint) uint64 {
	return ((x >> (y % uint64(w))) | (x << (uint64(w) - (y % uint64(w))))) & ((1 << w) - 1)
}
