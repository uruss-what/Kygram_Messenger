package algos

import (
	"encoding/binary"
	"errors"
)

const (
	BlockSize  = 16
	MaxKeySize = 32
)

type Twofish struct {
	key       []byte
	subKeys   [40]uint32
	sBoxes    [4][256]byte
	keyLength int
}

func NewTwofish() (*Twofish, error) {
	tf := &Twofish{}
	tf.generateSBoxes()

	key := []byte("securekey12345678")
	tf.key = key
	tf.keyLength = len(key)
	tf.generateSubKeys()

	// tf.CipherKey(tf.key)
	return tf, nil
}

func (tf *Twofish) CipherKey(key []byte) error {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return errors.New("invalid key size (must be 16, 24, or 32 bytes)")
	}
	tf.keyLength = len(tf.key)
	tf.generateSubKeys()

	return nil
}

func (tf *Twofish) generateSubKeys() {
	//fmt.Println("Расширяем ключ в tw хули")
	rsResult := rsEncode(tf.key)
	var evenKeys, oddKeys []uint32

	for i := 0; i < len(tf.key)/4; i++ {
		word := binary.LittleEndian.Uint32(tf.key[i*4 : (i+1)*4])
		if i%2 == 0 {
			evenKeys = append(evenKeys, word)
		} else {
			oddKeys = append(oddKeys, word)
		}
	}

	for i := 0; i < 20; i++ {
		a, b := pht(evenKeys[i%len(evenKeys)], oddKeys[i%len(oddKeys)])
		// tf.subKeys[2*i] = a + rsResult[0]
		// tf.subKeys[2*i+1] = b + rsResult[1]
		k := i % len(rsResult)
		tf.subKeys[2*i] = a + rsResult[k]
		tf.subKeys[2*i+1] = b + rsResult[(k+1)%len(rsResult)]
	}
}

func (tf *Twofish) generateSBoxes() {

	q0 := generateQ(t0, t1, t2, t3)
	q1 := generateQ(t0q1, t1q1, t2q1, t3q1)

	for i := 0; i < 256; i++ {
		tf.sBoxes[0][i] = applyQ(q0, byte(i))
		tf.sBoxes[1][i] = applyQ(q1, byte(i))
		tf.sBoxes[2][i] = applyQ(q0, applyQ(q1, byte(i)))
		tf.sBoxes[3][i] = applyQ(q1, applyQ(q0, byte(i)))
	}
}

func (tf *Twofish) HFunction(input uint32) uint32 {
	x0 := byte(input)
	x1 := byte(input >> 8)
	x2 := byte(input >> 16)
	x3 := byte(input >> 24)

	y0 := tf.sBoxes[0][x0]
	y1 := tf.sBoxes[1][x1]
	y2 := tf.sBoxes[2][x2]
	y3 := tf.sBoxes[3][x3]

	// MDS сразу нахуй ахах
	m0 := gfmultiply(y0, 0x01) ^ gfmultiply(y1, 0xEF) ^ gfmultiply(y2, 0x5B) ^ gfmultiply(y3, 0x5B)
	m1 := gfmultiply(y0, 0xEF) ^ gfmultiply(y1, 0x01) ^ gfmultiply(y2, 0xEF) ^ gfmultiply(y3, 0x5B)
	m2 := gfmultiply(y0, 0x5B) ^ gfmultiply(y1, 0xEF) ^ gfmultiply(y2, 0x01) ^ gfmultiply(y3, 0xEF)
	m3 := gfmultiply(y0, 0x5B) ^ gfmultiply(y1, 0x5B) ^ gfmultiply(y2, 0xEF) ^ gfmultiply(y3, 0x01)

	return (uint32(m0) << 24) | (uint32(m1) << 16) | (uint32(m2) << 8) | uint32(m3)
}

func gfmultiply(a, b byte) byte {
	var product byte = 0
	const irreducible = 0x69 // x⁸ + x⁶ + x⁵ + x³ + 1

	for i := 0; i < 8; i++ {
		if (b & 1) != 0 {
			product ^= a
		}

		highBit := a & 0x80
		a <<= 1

		if highBit != 0 {
			a ^= irreducible
		}

		b >>= 1
	}

	return product
}

func (tf *Twofish) Encrypt(data []byte) ([]byte, error) {
	//log.Println("Encrypt вызывается из Twofish")
	return tf.EncryptBlock(data)
}

func (tf *Twofish) Decrypt(data []byte) ([]byte, error) {
	return tf.DecryptBlock(data)
}

func (tf *Twofish) EncryptBlock(data []byte) ([]byte, error) {
	if len(data) != 16 {
		return nil, errors.New("invalid block size: must be 16 bytes")
	}

	var P [4]uint32
	for i := 0; i < 4; i++ {
		P[i] = uint32(data[i*4]) | uint32(data[i*4+1])<<8 | uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
	}

	P[0] ^= tf.subKeys[0]
	P[1] ^= tf.subKeys[1]
	P[2] ^= tf.subKeys[2]
	P[3] ^= tf.subKeys[3]

	for round := 0; round < 16; round++ {
		T0 := tf.HFunction(P[0])
		T1 := tf.HFunction(P[1])
		P[2] ^= T0 + T1 + tf.subKeys[2*round+8]
		P[3] ^= T0 + 2*T1 + tf.subKeys[2*round+9]

		if round < 15 {
			P[0], P[1], P[2], P[3] = P[2], P[3], P[0], P[1]
		}
	}

	P[0] ^= tf.subKeys[4]
	P[1] ^= tf.subKeys[5]
	P[2] ^= tf.subKeys[6]
	P[3] ^= tf.subKeys[7]

	ciphertext := make([]byte, 16)
	for i := 0; i < 4; i++ {
		ciphertext[i*4] = byte(P[i])
		ciphertext[i*4+1] = byte(P[i] >> 8)
		ciphertext[i*4+2] = byte(P[i] >> 16)
		ciphertext[i*4+3] = byte(P[i] >> 24)
	}

	return ciphertext, nil
}

func (tf *Twofish) DecryptBlock(data []byte) ([]byte, error) {
	if len(data) != 16 {
		return nil, errors.New("invalid block size: must be 16 bytes")
	}

	var P [4]uint32
	for i := 0; i < 4; i++ {
		P[i] = uint32(data[i*4]) | uint32(data[i*4+1])<<8 | uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
	}

	P[0] ^= tf.subKeys[4]
	P[1] ^= tf.subKeys[5]
	P[2] ^= tf.subKeys[6]
	P[3] ^= tf.subKeys[7]

	for round := 15; round >= 0; round-- {
		if round < 15 {
			P[0], P[1], P[2], P[3] = P[2], P[3], P[0], P[1]
		}

		T0 := tf.HFunction(P[0])
		T1 := tf.HFunction(P[1])
		P[2] ^= T0 + T1 + tf.subKeys[2*round+8]
		P[3] ^= T0 + 2*T1 + tf.subKeys[2*round+9]
	}

	P[0] ^= tf.subKeys[0]
	P[1] ^= tf.subKeys[1]
	P[2] ^= tf.subKeys[2]
	P[3] ^= tf.subKeys[3]

	plaintext := make([]byte, 16)
	for i := 0; i < 4; i++ {
		plaintext[i*4] = byte(P[i])
		plaintext[i*4+1] = byte(P[i] >> 8)
		plaintext[i*4+2] = byte(P[i] >> 16)
		plaintext[i*4+3] = byte(P[i] >> 24)
	}

	return plaintext, nil
}

var (
	t0 = [16]byte{8, 1, 7, 0xD, 6, 0xF, 3, 2, 0, 0xB, 5, 9, 0xE, 0xC, 0xA, 4}
	t1 = [16]byte{0xE, 0xC, 0xB, 8, 1, 2, 3, 5, 0xF, 4, 0xA, 6, 7, 0, 9, 0xD}
	t2 = [16]byte{0xB, 0xA, 5, 0xE, 6, 0xD, 9, 0, 0xC, 8, 0xF, 3, 2, 4, 7, 1}
	t3 = [16]byte{0xD, 7, 0xF, 4, 1, 2, 6, 0xE, 9, 0xB, 3, 0, 8, 5, 0xC, 0xA}
)

var (
	t0q1 = [16]byte{2, 8, 0xB, 0xD, 0xF, 7, 6, 0xE, 3, 1, 9, 4, 0, 0xA, 0xC, 5}
	t1q1 = [16]byte{1, 0xE, 2, 0xB, 4, 0xC, 3, 7, 6, 0xD, 0xA, 5, 0xF, 9, 0, 8}
	t2q1 = [16]byte{4, 0xC, 7, 5, 1, 6, 9, 0xA, 0, 0xE, 0xD, 8, 2, 0xB, 3, 0xF}
	t3q1 = [16]byte{0xB, 9, 5, 1, 0xC, 3, 0xD, 0xE, 6, 4, 7, 0xF, 2, 0, 8, 0xA}
)

var RSMatrix = [4][8]byte{
	{0x01, 0xa4, 0x55, 0x87, 0x5a, 0x58, 0xdb, 0x9e},
	{0xa4, 0x56, 0x82, 0xf3, 0x1e, 0xc6, 0x68, 0xe5},
	{0x02, 0xa1, 0xfc, 0xc1, 0x47, 0xae, 0x3d, 0x19},
	{0xa4, 0x55, 0x87, 0x5a, 0x58, 0xdb, 0x9e, 0x03},
}

func pht(a, b uint32) (uint32, uint32) {
	newA := a + b
	newB := a + 2*b
	return newA, newB
}

func rsEncode(key []byte) []uint32 {
	blockSize := 8
	numBlocks := len(key) / blockSize
	rsResult := make([]uint32, 0, numBlocks*4)

	for block := 0; block < numBlocks; block++ {
		start := block * blockSize
		end := start + blockSize
		chunk := key[start:end]

		var res [4]uint32
		for i := 0; i < 4; i++ {
			var sum byte
			for j := 0; j < 8; j++ {
				sum ^= gfmultiply(chunk[j], RSMatrix[i][j])
			}
			res[i] = uint32(sum)
		}
		combined := binary.LittleEndian.Uint32([]byte{byte(res[0]), byte(res[1]), byte(res[2]), byte(res[3])})
		rsResult = append(rsResult, combined)
	}

	return rsResult
}

func generateQ(t0, t1, t2, t3 [16]byte) [256]byte {
	var q [256]byte

	for x := 0; x < 256; x++ {
		a0 := byte(x / 16)
		b0 := byte(x % 16)
		a1 := a0 ^ b0
		b1 := a0 ^ ROR4(b0, 1) ^ (8*a0)%16
		a2 := t0[a1]
		b2 := t1[b1]
		a3 := a2 ^ b2
		b3 := a2 ^ ROR4(b2, 1) ^ (8*a2)%16
		a4 := t2[a3]
		b4 := t3[b3]
		q[x] = 16*b4 + a4
	}

	return q
}
func ROR4(x, n byte) byte {
	return (x >> n) | (x<<(4-n))&0x0F
}

func applyQ(q [256]byte, x byte) byte {
	return q[x]
}
