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
	createConn := func() (net.Conn, *bufio.Reader, error) {
		badC, err := net.Dial("tcp", badProxyUrl.Host)
		checkError(err)

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
			headers = append(headers, fmt.Sprintf("Proxy-Authorization:Basic %s", auth))
		}
		fmt.Fprint(badC, writeHead(headers))
		head, bufRd := readHead(badC)
		if head[0] != "HTTP/1.1 101 Switching Protocols" {
			badC.Close()
			return nil, nil, errors.New("Unexpected server response.")
		}

		return badC, bufRd, nil
	}

	listen(localPort, remotePort, conf, createConn)
}

func listen(localPort, remotePort string, conf env, createConn func() (net.Conn, *bufio.Reader, error)) {
	l, err := net.Listen("tcp", fmt.Sprintf(":%s", localPort))
	checkError(err)
	defer l.Close()

	for {
		c, err := l.Accept()
		checkError(err)
		debugInfo(conf.debug, "Client: received request.")

		go func() {
			badC, bufRd, err := createConn()
			if err != nil {
				c.Close()
				return
			}

			// communicate IV
			iv := genIV()
			fmt.Fprintf(badC, writeHead([]string{fmt.Sprintf("%x", iv)}))

			wrappedWriter := wrapWriter(badC, conf.pwd, iv)
			wrappedReader := wrapReader(bufRd, conf.pwd, iv)

			// tell server which port to forward to
			fmt.Fprintf(wrappedWriter, writeHead([]string{fmt.Sprintf("%s", remotePort)}))

			go io.Copy(wrappedWriter, c)
			io.Copy(c, wrappedReader)
			c.Close()
			badC.Close()
		}()
	}
}
