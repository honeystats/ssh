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

func genHostKey() (*rsa.PrivateKey, error) {
	var r SeededRandReader
	r = SeededRandReader{
		Random: rand.New(rand.NewSource(11)),
	}

	return rsa.GenerateKey(r, 2048)
}
