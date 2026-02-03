package random

import (
	"crypto/rand"
	"encoding/base64"
	"math/big"
)

var (
	numSeq      [10]rune
	lowerSeq    [26]rune
	upperSeq    [26]rune
	numLowerSeq [36]rune
	numUpperSeq [36]rune
	allSeq      [62]rune
)

func init() {
	for i := 0; i < 10; i++ {
		numSeq[i] = rune('0' + i)
	}
	for i := 0; i < 26; i++ {
		lowerSeq[i] = rune('a' + i)
		upperSeq[i] = rune('A' + i)
	}

	copy(numLowerSeq[:], numSeq[:])
	copy(numLowerSeq[len(numSeq):], lowerSeq[:])

	copy(numUpperSeq[:], numSeq[:])
	copy(numUpperSeq[len(numSeq):], upperSeq[:])

	copy(allSeq[:], numSeq[:])
	copy(allSeq[len(numSeq):], lowerSeq[:])
	copy(allSeq[len(numSeq)+len(lowerSeq):], upperSeq[:])
}

func Seq(n int) string {
	runes := make([]rune, n)
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(allSeq))))
		if err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
		runes[i] = allSeq[idx.Int64()]
	}
	return string(runes)
}

// Num generates a random integer between 0 and n-1.
func Num(n int) int {
	if n <= 0 {
		return 0
	}
	bn := big.NewInt(int64(n))
	r, err := rand.Int(rand.Reader, bn)
	if err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return int(r.Int64())
}

// LowerNumSeq 生成指定长度的小写字母+数字随机字符串
func LowerNumSeq(n int) string {
	runes := make([]rune, n)
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(numLowerSeq))))
		if err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
		runes[i] = numLowerSeq[idx.Int64()]
	}
	return string(runes)
}

// SeqWithCharset 生成指定字符集的随机字符串
func SeqWithCharset(n int, charset string) string {
	if len(charset) == 0 {
		return ""
	}
	runes := []rune(charset)
	result := make([]rune, n)
	for i := 0; i < n; i++ {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(runes))))
		if err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
		result[i] = runes[idx.Int64()]
	}
	return string(result)
}

// Base64Bytes 生成 n 字节随机数据并返回 base64 编码
func Base64Bytes(n int) string {
	bytes := make([]byte, n)
	_, err := rand.Read(bytes)
	if err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
