package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net"
)

func handle(incomingC net.Conn, bufRd *bufio.Reader, conf env) {
	ivString, _ := readHead(bufRd)
	iv, _ := hex.DecodeString(ivString[0])
	debugInfof(conf.debug, "IV received: %s\n", ivString)

	wrappedWriter := wrapWriter(incomingC, conf.pwd, iv)
	wrappedReader := wrapReader(bufRd, conf.pwd, iv)

	content, bufferedWrappedReader := readHead(wrappedReader)
	remotePort := content[0]
	debugInfof(conf.debug, "Port forwarded to %s\n", remotePort)
	localC, err := net.Dial("tcp", "127.0.0.1:"+remotePort)
	if err != nil {
		fmt.Println(err)
		return
	}
	go io.Copy(localC, bufferedWrappedReader)
	io.Copy(wrappedWriter, localC)
	incomingC.Close()
	localC.Close()
}

func startServer(port string, conf env) {
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		go func() {
			head, bufRd := readHead(c)
			isWstunnel := false
			for _, i := range head {
				if i == "User-Agent: wstunnel-go" {
					isWstunnel = true
					break
				}
			}

			if isWstunnel {
				debugInfo(conf.debug, "Server: received request.")
				fmt.Fprintf(c, writeHead([]string{
					"HTTP/1.1 101 Switching Protocols",
					"Upgrade: websocket",
					"Connection: Upgrade",
				}))
				handle(c, bufRd, conf)
			} else {
				fmt.Fprintf(c, writeHead([]string{
					"HTTP/1.1 400 Bad Request",
				}))
				c.Close()
			}
		}()
	}
}
