package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"strings"
)

func checkError(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func writeHead(parts []string) string {
	return strings.Join(parts, "\r\n") + "\r\n\r\n"
}

func readHead(rd io.Reader) ([]string, *bufio.Reader) {
	x, ok := (rd).(*bufio.Reader)
	var reader *bufio.Reader
	if ok {
		reader = x
	} else {
		reader = bufio.NewReader(rd)
	}

	res := make([]string, 0)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Failed to read new line.", err)
			return res, nil
		}
		line = strings.Replace(line, "\r\n", "", 1)
		if line == "" {
			break
		}
		res = append(res, line)
	}
	return res, reader
}

func debugInfo(debug bool, a ...interface{}) {
	if debug {
		fmt.Println(a...)
	}
}

func debugInfof(debug bool, format string, a ...interface{}) {
	if debug {
		fmt.Printf(format, a...)
	}
}

func wrapReader(rd io.Reader, pwd string, iv []byte) io.Reader {
	if pwd == "" {
		return rd
	}
	return &cipher.StreamReader{S: genStream(pwd, iv), R: rd}
}

func wrapWriter(wt io.Writer, pwd string, iv []byte) io.Writer {
	if pwd == "" {
		return wt
	}
	return &cipher.StreamWriter{S: genStream(pwd, iv), W: wt}
}

func genStream(pwd string, iv []byte) cipher.Stream {
	hasher := sha1.New()
	hasher.Write([]byte(pwd))
	key := hasher.Sum(nil)[:32]

	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	stream := cipher.NewOFB(block, iv[:])
	return stream
}

func genIV() []byte {
	var iv [aes.BlockSize]byte
	_, err := io.ReadFull(rand.Reader, iv[:])
	if err != nil {
		panic(err)
	}
	return iv[:]
}
