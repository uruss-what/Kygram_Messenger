package algos

import (
	"crypto/rand"
	"crypto/sha256"
	"math/big"
)

func GeneratePrime(bits int) (*big.Int, error) {
	prime, err := rand.Prime(rand.Reader, bits)
	if err != nil {
		return nil, err
	}
	return prime, nil
}

func GeneratePrivateKey(prime *big.Int) (*big.Int, error) {
	privateKey, err := rand.Int(rand.Reader, prime)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

func GeneratePublicKey(g, privateKey, prime *big.Int) *big.Int {
	publicKey := new(big.Int).Exp(g, privateKey, prime)
	return publicKey
}

func GenerateSharedKey(privateKey, otherPublicKey, prime *big.Int) *big.Int {
	sharedKey := new(big.Int).Exp(otherPublicKey, privateKey, prime)
	return sharedKey
}

func HashSharedKey(sharedKey *big.Int) []byte {
	hash := sha256.New()
	hash.Write(sharedKey.Bytes())
	hashedKey := hash.Sum(nil)
	return hashedKey
}
