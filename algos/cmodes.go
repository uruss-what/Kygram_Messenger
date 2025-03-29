package algos

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"log"
	"sync"
)

type EncryptionMode int

const (
	ECB EncryptionMode = iota
	CBC
	PCBC
	CFB
	OFB
	CTR
	RandomDelta
)

type PaddingMode int

const (
	Zeros PaddingMode = iota
	ANSI_X_923
	PKCS7
	ISO_10126
)

type EncryptionContext struct {
	Key            []byte
	roundKeys      [][]byte
	Mode           EncryptionMode
	Padding        PaddingMode
	IV             []byte
	AdditionalArgs []interface{}
	Cipher         Cipher
	expanderKey    KeyExpander
	mu             sync.Mutex
}

func NewEncryptionContext(key []byte, mode EncryptionMode, padding PaddingMode, iv []byte, cipher Cipher, expanderKey KeyExpander, additionalArgs ...interface{}) *EncryptionContext {

	var roundKeys [][]byte
	if rc5, ok := cipher.(*RC5); ok {
		roundKeys = convertUint64ToByteSlices(rc5.keys)
	}
	if tw, ok := cipher.(*Twofish); ok {
		roundKeys = convertUint32ToByteSlices(tw.subKeys[:])
	}
	ctx := &EncryptionContext{
		Cipher:         cipher,
		Key:            key,
		Mode:           mode,
		Padding:        padding,
		IV:             iv,
		AdditionalArgs: additionalArgs,
		expanderKey:    expanderKey,
		roundKeys:      roundKeys,
	}

	if err := ctx.CipherKey(key); err != nil {
		log.Printf("failed to set context cipher key: %v", err)
	}

	return ctx
}

func convertUint64ToByteSlices(keys []uint64) [][]byte {
	result := make([][]byte, len(keys))
	for i, key := range keys {
		keyBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(keyBytes, key)
		result[i] = keyBytes
	}
	return result
}
func convertUint32ToByteSlices(keys []uint32) [][]byte {
	result := make([][]byte, len(keys))
	for i, key := range keys {
		keyBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(keyBytes, key)
		result[i] = keyBytes
	}
	return result
}

type Cipher interface {
	CipherKey(key []byte) error
	Encrypt(inputBlock []byte) ([]byte, error)
	Decrypt(inputBlock []byte) ([]byte, error)
}

type KeyExpander interface {
	ExpandKey(key []byte) ([][]byte, error)
}

func (r *RC5) ExpandKey(key []byte) ([][]byte, error) {
	if len(key) < 1 {
		return nil, errors.New("key can not be empty")
	}

	expandedKeys := r.keyExpansion(key)

	roundKeys := make([][]byte, len(expandedKeys))
	for i, k := range expandedKeys {
		roundKeys[i] = make([]byte, 8)
		binary.LittleEndian.PutUint64(roundKeys[i], k)
	}
	return roundKeys, nil
}

func (tf *Twofish) ExpandKey(key []byte) ([][]byte, error) {
	if len(key) < 1 {
		return nil, errors.New("key can not be empty")
	}

	// tf.key = key
	// tf.keyLength = len(key)
	// tf.generateSubKeys()

	roundKeys := make([][]byte, len(tf.subKeys))
	for i, k := range tf.subKeys {
		roundKeys[i] = make([]byte, 4)
		binary.LittleEndian.PutUint32(roundKeys[i], k)
	}

	return roundKeys, nil
}

func (ctx *EncryptionContext) CipherKey(key []byte) error {
	if ctx.expanderKey == nil {
		return errors.New("invalid KeyExpander")
	}

	roundKeys, err := ctx.expanderKey.ExpandKey(key)
	if err != nil {
		return err
	}

	ctx.roundKeys = roundKeys
	return nil
}

func (ctx *EncryptionContext) addPadding(input []byte, paddingMode PaddingMode) ([]byte, error) {
	blockSize := len(ctx.IV)
	paddingNeeded := blockSize - len(input)
	if paddingNeeded == 0 {
		paddingNeeded = blockSize
	}

	switch paddingMode {
	case Zeros:
		return zeroPadding(input, blockSize), nil //done
	case PKCS7:
		return pkcs7Padding(input, blockSize), nil //done
	case ANSI_X_923:
		return ansix923Padding(input, blockSize), nil //done
	case ISO_10126:
		return iso10126Padding(input, blockSize) //crash
	default:
		return nil, errors.New("error of applying padding mode")
	}
}

func (ctx *EncryptionContext) removePadding(input []byte, paddingMode PaddingMode) ([]byte, error) {
	switch paddingMode {
	case Zeros:
		return zeroUnpadding(input), nil
	case PKCS7:
		return pkcs7Unpadding(input)
	case ANSI_X_923:
		return ansix923Unpadding(input)
	case ISO_10126:
		return iso10126Unpadding(input)
	default:
		return nil, errors.New("error of removing padding mode")
	}
}

func zeroPadding(input []byte, blockSize int) []byte {
	padSize := blockSize - len(input)%blockSize
	return append(input, bytes.Repeat([]byte{0}, padSize)...)
}

func zeroUnpadding(input []byte) []byte {
	return bytes.TrimRightFunc(input, func(r rune) bool {
		return r == 0
	})
}

func pkcs7Padding(input []byte, blockSize int) []byte {
	if blockSize <= 0 {
		panic("blockSize must be greater than zero")
	}
	padSize := blockSize - len(input)%blockSize
	pad := bytes.Repeat([]byte{byte(padSize)}, padSize)
	return append(input, pad...)
}

func pkcs7Unpadding(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("input is empty")
	}
	padSize := int(input[len(input)-1])
	if padSize > len(input) {
		return nil, errors.New("invalid padding size")
	}
	return input[:len(input)-padSize], nil
}

func ansix923Padding(input []byte, blockSize int) []byte {
	if blockSize <= 0 {
		panic("blockSize must be greater than zero")
	}
	// padding := append(bytes.Repeat([]byte{0}, blockSize-1), byte(blockSize))
	paddingSize := blockSize - len(input)%blockSize
	if paddingSize == 0 {
		paddingSize = blockSize
	}
	padding := append(bytes.Repeat([]byte{0}, paddingSize-1), byte(paddingSize))
	return append(input, padding...)
}

func ansix923Unpadding(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("input is empty")
	}
	blockSize := int(input[len(input)-1])
	if blockSize > len(input) || blockSize == 0 {
		return nil, errors.New("invalid padding size")
	}
	for _, v := range input[len(input)-blockSize : len(input)-1] {
		if v != 0 {
			return nil, errors.New("invalid ansi_X_923 padding")
		}
	}
	return input[:len(input)-blockSize], nil
}

func iso10126Padding(input []byte, blockSize int) ([]byte, error) {
	paddingSize := blockSize - len(input)%blockSize
	if paddingSize == 0 {
		paddingSize = blockSize
	}
	padding := make([]byte, paddingSize-1)
	_, err := rand.Read(padding)
	if err != nil {
		return nil, errors.New("error appliyng random padding iso10126")
	}
	return append(input, append(padding, byte(paddingSize))...), nil
}

func iso10126Unpadding(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("input is empty")
	}
	blockSize := int(input[len(input)-1])
	if blockSize > len(input) || blockSize == 0 {
		return nil, errors.New("invalid padding size")
	}
	return input[:len(input)-blockSize], nil
}

func (ctx *EncryptionContext) Encrypt(input []byte) ([]byte, error) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	paddedData, err := ctx.addPadding(input, ctx.Padding)
	if err != nil {
		return nil, err
	}

	switch ctx.Mode {
	case CBC:
		return encryptWithCBC(ctx, paddedData)
	case ECB:
		return encryptWithECB(ctx, paddedData)
	case PCBC:
		return encryptWithPCBC(ctx, paddedData)
	case CFB:
		return encryptWithCFB(ctx, paddedData)
	case OFB:
		return encryptWithOFB(ctx, paddedData)
	case CTR:
		return encryptWithCTR(ctx, paddedData)
	case RandomDelta:
		return encryptWithRandomDelta(ctx, paddedData)
	default:
		return nil, errors.New("invalid cipher mode")
	}
}

func (ctx *EncryptionContext) Decrypt(input []byte) ([]byte, error) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	var decryptedData []byte
	var err error

	switch ctx.Mode {
	case CBC:
		decryptedData, err = decryptWithCBC(ctx, input)
	case ECB:
		decryptedData, err = decryptWithECB(ctx, input)
	case PCBC:
		decryptedData, err = decryptWithPCBC(ctx, input)
	case CFB:
		decryptedData, err = decryptWithCFB(ctx, input)
	case OFB:
		decryptedData, err = decryptWithOFB(ctx, input)
	case CTR:
		decryptedData, err = decryptWithCTR(ctx, input)
	case RandomDelta:
		decryptedData, err = decryptWithRandomDelta(ctx, input)
	default:
		return nil, errors.New("invalid cipher mode")
	}
	if err != nil {
		return nil, err
	}

	return decryptedData, nil
}

func xorBlocks(block1, block2 []byte) []byte {
	xored := make([]byte, len(block1))
	for i := range block1 {
		xored[i] = block1[i] ^ block2[i]
	}
	return xored
}

func encryptWithCBC(ctx *EncryptionContext, input []byte) ([]byte, error) {
	log.Printf("Данные перед шифрованием в CBC: %q\n", input)

	blockSize := len(ctx.IV)
	// if blockSize == 0 || len(input)%blockSize != 0 {
	// 	return nil, errors.New("input must be a multiple of the block size")
	// }

	numBlocks := len(input) / blockSize
	encrypted := make([]byte, len(input))
	previousBlock := ctx.IV

	for i := 0; i < numBlocks; i++ {
		block := input[i*blockSize : (i+1)*blockSize]
		xoredBlock := xorBlocks(block, previousBlock)
		//fmt.Printf("xoredBlock (CBC): %q\n", xoredBlock)

		cipherBlock, err := ctx.Cipher.Encrypt(xoredBlock)
		if err != nil {
			return nil, err
		}

		copy(encrypted[i*blockSize:], cipherBlock)
		previousBlock = cipherBlock
	}
	log.Printf("Зашифрованные данные (CBC): %q\n", encrypted)
	return encrypted, nil
}

func decryptWithCBC(ctx *EncryptionContext, input []byte) ([]byte, error) {
	blockSize := len(ctx.IV)
	if blockSize == 0 || len(input)%blockSize != 0 {
		return nil, errors.New("input must be a multiple of the block size")
	}

	numBlocks := len(input) / blockSize
	decrypted := make([]byte, len(input))
	previousBlock := ctx.IV

	for i := 0; i < numBlocks; i++ {
		block := input[i*blockSize : (i+1)*blockSize]

		plainBlock, err := ctx.Cipher.Decrypt(block)
		if err != nil {
			return nil, err
		}

		xoredBlock := xorBlocks(plainBlock, previousBlock)
		copy(decrypted[i*blockSize:], xoredBlock)

		previousBlock = block
	}

	return ctx.removePadding(decrypted, ctx.Padding)
}

func encryptWithECB(ctx *EncryptionContext, input []byte) ([]byte, error) {
	// log.Printf("Данные перед шифрованием в ECB: %q\n", input)
	blockSize := len(ctx.IV)
	if blockSize == 0 || len(input)%blockSize != 0 {
		return nil, errors.New("input must be a multiple of the block size")
	}

	numBlocks := len(input) / blockSize
	encrypted := make([]byte, len(input))

	for i := 0; i < numBlocks; i++ {
		block := input[i*blockSize : (i+1)*blockSize]
		cipherBlock, err := ctx.Cipher.Encrypt(block)
		if err != nil {
			return nil, err
		}
		copy(encrypted[i*blockSize:], cipherBlock)
	}

	return encrypted, nil
}

func decryptWithECB(ctx *EncryptionContext, input []byte) ([]byte, error) {
	blockSize := len(ctx.IV)
	if len(input)%blockSize != 0 {
		return nil, errors.New("input length must be a multiple of the block size")
	}

	numBlocks := len(input) / blockSize
	decrypted := make([]byte, len(input))

	for i := 0; i < numBlocks; i++ {
		block := input[i*blockSize : (i+1)*blockSize]
		plainBlock, err := ctx.Cipher.Decrypt(block)
		if err != nil {
			return nil, err
		}
		copy(decrypted[i*blockSize:], plainBlock)
	}

	return ctx.removePadding(decrypted, ctx.Padding)
}

func encryptWithPCBC(ctx *EncryptionContext, input []byte) ([]byte, error) {
	log.Printf("Данные перед шифрованием в PCBC: %q\n", input)
	blockSize := len(ctx.IV)
	if len(input)%blockSize != 0 {
		return nil, errors.New("input length must be a multiple of the block size")
	}

	numBlocks := len(input) / blockSize
	encrypted := make([]byte, len(input))
	previousCipherBlock := ctx.IV

	for i := 0; i < numBlocks; i++ {
		plainBlock := input[i*blockSize : (i+1)*blockSize]
		xorBlock := xorBlocks(plainBlock, previousCipherBlock)

		cipherBlock, err := ctx.Cipher.Encrypt(xorBlock)
		if err != nil {
			return nil, err
		}

		copy(encrypted[i*blockSize:], cipherBlock)

		previousCipherBlock = xorBlocks(plainBlock, cipherBlock)
	}

	return encrypted, nil
}

func decryptWithPCBC(ctx *EncryptionContext, input []byte) ([]byte, error) {
	blockSize := len(ctx.IV)
	if len(input)%blockSize != 0 {
		return nil, errors.New("input length must be a multiple of the block size")
	}

	numBlocks := len(input) / blockSize
	decrypted := make([]byte, len(input))
	previousCipherBlock := ctx.IV

	for i := 0; i < numBlocks; i++ {
		cipherBlock := input[i*blockSize : (i+1)*blockSize]

		xorBlock, err := ctx.Cipher.Decrypt(cipherBlock)
		if err != nil {
			return nil, err
		}

		plainBlock := xorBlocks(xorBlock, previousCipherBlock)
		copy(decrypted[i*blockSize:], plainBlock)

		previousCipherBlock = xorBlocks(plainBlock, cipherBlock)
	}

	return ctx.removePadding(decrypted, ctx.Padding)
}

func encryptWithCFB(ctx *EncryptionContext, input []byte) ([]byte, error) {
	log.Printf("Данные перед шифрованием в CFB: %q\n", input)
	blockSize := len(ctx.IV)
	if len(input)%blockSize != 0 {
		return nil, errors.New("input length must be a multiple of the block size")
	}

	numBlocks := len(input) / blockSize
	encrypted := make([]byte, len(input))
	feedbackBlock := ctx.IV

	for i := 0; i < numBlocks; i++ {
		plainBlock := input[i*blockSize : (i+1)*blockSize]

		cipherFeedback, err := ctx.Cipher.Encrypt(feedbackBlock)
		if err != nil {
			return nil, err
		}

		cipherBlock := xorBlocks(plainBlock, cipherFeedback)
		copy(encrypted[i*blockSize:], cipherBlock)

		feedbackBlock = cipherFeedback
	}

	return encrypted, nil
}

func decryptWithCFB(ctx *EncryptionContext, input []byte) ([]byte, error) {
	blockSize := len(ctx.IV)
	if len(input)%blockSize != 0 {
		return nil, errors.New("input length must be a multiple of the block size")
	}

	numBlocks := len(input) / blockSize
	decrypted := make([]byte, len(input))
	feedbackBlock := ctx.IV

	for i := 0; i < numBlocks; i++ {
		cipherBlock := input[i*blockSize : (i+1)*blockSize]

		cipherFeedback, err := ctx.Cipher.Encrypt(feedbackBlock)
		if err != nil {
			return nil, err
		}

		plainBlock := xorBlocks(cipherBlock, cipherFeedback)
		copy(decrypted[i*blockSize:], plainBlock)

		feedbackBlock = cipherBlock
	}

	return ctx.removePadding(decrypted, ctx.Padding)
}

func encryptWithOFB(ctx *EncryptionContext, input []byte) ([]byte, error) {
	log.Printf("Данные перед шифрованием в OFB: %q\n", input)
	blockSize := len(ctx.IV)
	if len(input)%blockSize != 0 {
		return nil, errors.New("input length must be a multiple of the block size")
	}

	_, err := ctx.addPadding(input, ctx.Padding)
	if err != nil {
		return nil, err
	}

	numBlocks := len(input) / blockSize
	encrypted := make([]byte, len(input))
	feedbackBlock := ctx.IV

	for i := 0; i < numBlocks; i++ {
		feedbackBlock, err = ctx.Cipher.Encrypt(feedbackBlock)
		if err != nil {
			return nil, err
		}

		plainBlock := input[i*blockSize : (i+1)*blockSize]
		cipherBlock := xorBlocks(plainBlock, feedbackBlock)
		copy(encrypted[i*blockSize:], cipherBlock)

	}

	return encrypted, nil
}

func decryptWithOFB(ctx *EncryptionContext, input []byte) ([]byte, error) {
	blockSize := len(ctx.IV)
	if len(input)%blockSize != 0 {
		return nil, errors.New("input length must be a multiple of the block size")
	}

	numBlocks := len(input) / blockSize
	decrypted := make([]byte, len(input))
	feedbackBlock := ctx.IV

	for i := 0; i < numBlocks; i++ {
		feedbackBlock, err := ctx.Cipher.Encrypt(feedbackBlock)
		if err != nil {
			return nil, err
		}

		cipherBlock := input[i*blockSize : (i+1)*blockSize]
		plainBlock := xorBlocks(cipherBlock, feedbackBlock)
		copy(decrypted[i*blockSize:], plainBlock)

	}

	return ctx.removePadding(decrypted, ctx.Padding)
}

func encryptWithCTR(ctx *EncryptionContext, input []byte) ([]byte, error) {
	log.Printf("Данные перед шифрованием в CTR: %q\n", input)
	blockSize := len(ctx.IV)
	if len(input)%blockSize != 0 {
		return nil, errors.New("input length must be a multiple of the block size")
	}

	numBlocks := len(input) / blockSize
	encrypted := make([]byte, len(input))

	counter := append([]byte(nil), ctx.IV...)

	for i := 0; i < numBlocks; i++ {
		encryptedCounter, err := ctx.Cipher.Encrypt(counter)
		if err != nil {
			return nil, err
		}

		plainBlock := input[i*blockSize : (i+1)*blockSize]
		cipherBlock := xorBlocks(plainBlock, encryptedCounter)
		copy(encrypted[i*blockSize:], cipherBlock)

		counter = incrementCounter(counter)
	}

	return encrypted, nil
}

func decryptWithCTR(ctx *EncryptionContext, input []byte) ([]byte, error) {
	return encryptWithCTR(ctx, input)
}

func incrementCounter(counter []byte) []byte {
	for i := len(counter) - 1; i >= 0; i-- {
		counter[i]++
		if counter[i] != 0 {
			break
		}
	}
	return counter
}

func encryptWithRandomDelta(ctx *EncryptionContext, input []byte) ([]byte, error) {
	log.Printf("Данные перед шифрованием в RandomDelta: %q\n", input)
	blockSize := len(ctx.IV)
	if len(input)%blockSize != 0 {
		return nil, errors.New("input length must be a multiple of the block size")
	}

	numBlocks := len(input) / blockSize
	encrypted := make([]byte, len(input))

	deltaStr := "7e8f9d52b43c0818d5e8a09c72fbbacf"

	delta, err := hex.DecodeString(deltaStr)
	if err != nil {
		return nil, err
	}

	for i := 0; i < numBlocks; i++ {
		plainBlock := input[i*blockSize : (i+1)*blockSize]
		blockWithDelta := xorBlocks(plainBlock, delta)

		encryptedBlock, err := ctx.Cipher.Encrypt(blockWithDelta)
		if err != nil {
			return nil, err
		}

		copy(encrypted[i*blockSize:], encryptedBlock)
	}

	return encrypted, nil
}

func decryptWithRandomDelta(ctx *EncryptionContext, input []byte) ([]byte, error) {
	blockSize := len(ctx.IV)
	if len(input)%blockSize != 0 {
		return nil, errors.New("input length must be a multiple of the block size")
	}

	numBlocks := len(input) / blockSize
	decrypted := make([]byte, len(input))

	deltaStr := "7e8f9d52b43c0818d5e8a09c72fbbacf"

	delta, err := hex.DecodeString(deltaStr)
	if err != nil {
		return nil, err
	}

	for i := 0; i < numBlocks; i++ {
		encryptedBlock := input[i*blockSize : (i+1)*blockSize]
		blockWithDelta, err := ctx.Cipher.Decrypt(encryptedBlock)
		if err != nil {
			return nil, err
		}

		plainBlock := xorBlocks(blockWithDelta, delta)
		copy(decrypted[i*blockSize:], plainBlock)
	}

	return decrypted, nil
}
