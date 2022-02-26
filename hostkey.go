package main

import (
	"crypto/rsa"
	"math/rand"
)

type SeededRandReader struct {
	Random *rand.Rand
}

func (srr SeededRandReader) Read(buf []byte) (n int, err error) {
	return srr.Random.Read(buf)
}

func strDigest(str string) int64 {
	var ret int64
	for _, ch := range str {
		ret += int64(ch)
	}
	return ret
}

func genHostKey(seedStr string) (*rsa.PrivateKey, error) {
	var r SeededRandReader
	r = SeededRandReader{
		Random: rand.New(
			rand.NewSource(strDigest(seedStr)),
		),
	}

	return rsa.GenerateKey(r, 2048)
}
