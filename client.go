package main

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
)

func startClient(localPort, remotePort string, serverUrl, badProxyUrl *url.URL, conf env) {
	activeCount := 0 // badC

	createConn := func() (net.Conn, *bufio.Reader, error) {
		badC, err := net.Dial("tcp", badProxyUrl.Host)
		if err != nil {
			return nil, nil, err
		}

		// upgrade to websocket
		// doesn't have to be real websocket
		// as long as it looks like websocket to bad proxy
		// all we need is a bi-directional channel
		headers := []string{
			fmt.Sprintf("GET ws://%s/ HTTP/1.1", serverUrl.Host),
			fmt.Sprintf("Host: %s", serverUrl.Host),
			"Connection: Upgrade",
			"Pragma: no-cache",
			"Cache-Control: no-cache",
			"Upgrade: websocket",
			"User-Agent: wstunnel-go",
		}
		auth := base64.StdEncoding.EncodeToString([]byte(badProxyUrl.User.String()))
		if auth != "" {
			headers = append(headers, fmt.Sprintf("Proxy-Authorization: Basic %s", auth))
		}
		fmt.Fprint(badC, writeHead(headers))
		head, bufRd := readHead(badC)
		if head[0] != "HTTP/1.1 101 Switching Protocols" {
			badC.Close()
			return nil, nil, errors.New("Failed to upgrade to websocket.")
		}

		return badC, bufRd, nil
	}

	listen := func() {
		l, err := net.Listen("tcp", ":"+localPort)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer l.Close()

		for {
			// client <=> user
			localC, err := l.Accept()
			if err != nil {
				fmt.Println(err)
				return
			}

			go func() {
				// client <=> bad proxy
				badC, bufRd, err := createConn()
				if err != nil {
					localC.Close()
					return
				}

				activeCount++
        debugInfof(conf.debug, "local: %s ---> remote: %s\nClient: current connection #: %d\n", localPort, remotePort, activeCount)

				// communicate IV
				iv := genIV()
				fmt.Fprintf(badC, writeHead([]string{fmt.Sprintf("%x", iv)}))

				wrappedWriter := wrapWriter(badC, conf.pwd, iv)
				wrappedReader := wrapReader(bufRd, conf.pwd, iv)

				// tell server which port to forward to
				fmt.Fprintf(wrappedWriter, writeHead([]string{fmt.Sprintf("%s", remotePort)}))

				go io.Copy(localC, wrappedReader)
				io.Copy(wrappedWriter, localC)
				localC.Close()
				badC.Close()
				debugInfo(conf.debug, "Client: connection closed.")
				activeCount--
			}()
		}
	}

	listen()
}
