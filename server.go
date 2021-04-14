package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"net"
)

func startServer(port string, conf env) {
	activeCount := 0 // incomingC

	handle := func(incomingC net.Conn, bufRd *bufio.Reader, conf env) {
		ivString, _ := readHead(bufRd)
		iv, _ := hex.DecodeString(ivString[0])
		debugInfof(conf.debug, "IV received: %s\n", ivString)

		wrappedWriter := wrapWriter(incomingC, conf.pwd, iv)
		wrappedReader := wrapReader(bufRd, conf.pwd, iv)

		content, bufferedWrappedReader := readHead(wrappedReader)
		if len(content) == 0 {
			incomingC.Close()
			activeCount--
			return
		}
		remotePort := content[0]
		debugInfof(conf.debug, "Port forwarded to %s\n", remotePort)
		// server <=> destination process
		localC, err := net.Dial("tcp", "127.0.0.1:"+remotePort)
		if err != nil {
			fmt.Println(err)
			incomingC.Close()
			activeCount--
			return
		}
		go io.Copy(wrappedWriter, localC)
		io.Copy(localC, bufferedWrappedReader)
		incomingC.Close()
		localC.Close()
		debugInfo(conf.debug, "Server: connection closed.")
		activeCount--
	}

	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer l.Close()

	for {
		// server <=> proxy
		incomingC, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}

		activeCount++
		debugInfof(conf.debug, "Server: current connection #: %d\n", activeCount)

		go func() {
			head, bufRd := readHead(incomingC)
			isWstunnel := false
			for _, i := range head {
				if i == "User-Agent: wstunnel-go" {
					isWstunnel = true
					break
				}
			}

			if isWstunnel {
				debugInfo(conf.debug, "Server: received request.")
				fmt.Fprintf(incomingC, writeHead([]string{
					"HTTP/1.1 101 Switching Protocols",
					"Upgrade: websocket",
					"Connection: Upgrade",
				}))
				handle(incomingC, bufRd, conf)
			} else {
				fmt.Fprintf(incomingC, writeHead([]string{
					"HTTP/1.1 400 Bad Request",
				}))
				incomingC.Close()
				activeCount--
			}
		}()
	}
}
