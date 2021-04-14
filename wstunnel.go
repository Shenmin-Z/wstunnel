package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
)

type env struct {
	pwd   string // key for aes
	debug bool   // display debug info
}

func main() {
	clientCmd := flag.NewFlagSet("client", flag.ExitOnError)
	serverCmd := flag.NewFlagSet("server", flag.ExitOnError)

	var (
		pf           = ""
		serverAddr   = ""
		port         = ""
		badProxyAddr = ""
		pwd          = ""
		debug        = false
	)
	clientCmd.StringVar(&pf, "port-forward", "", "port forward: {local1}:{remote1};{local2}:{remote2}")
	clientCmd.StringVar(&pf, "f", "", "port forward: {local1}:{remote1};{local2}:{remote2}")
	clientCmd.StringVar(&serverAddr, "server", "", "address where server process is, e.g. http://1.1.1.1:8080")
	clientCmd.StringVar(&serverAddr, "s", "", "address where server process is, e.g. http://1.1.1.1:8080")
	clientCmd.StringVar(&badProxyAddr, "proxy", "", "proxy, e.g. http://user:password@1.1.1.1:8080")
	clientCmd.StringVar(&badProxyAddr, "p", "", "proxy, e.g. http://user:password@1.1.1.1:8080")
	clientCmd.StringVar(&pwd, "password", "", "password for encrytion")
	clientCmd.StringVar(&pwd, "w", "", "password for encrytion")
	clientCmd.BoolVar(&debug, "debug", false, "show debug info")
	clientCmd.BoolVar(&debug, "d", false, "show debug info")

	serverCmd.StringVar(&port, "port", "", "port on which the prcess is listening on")
	serverCmd.StringVar(&port, "p", "", "port on which the prcess is listening on")
	serverCmd.StringVar(&pwd, "password", "", "password for encrytion")
	serverCmd.StringVar(&pwd, "w", "", "password for encrytion")
	serverCmd.BoolVar(&debug, "debug", false, "show debug info")
	serverCmd.BoolVar(&debug, "d", false, "show debug info")

	if len(os.Args) < 2 {
		fmt.Println("Need to specify: client or server")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "client":
		clientCmd.Parse(os.Args[2:])
	case "server":
		serverCmd.Parse(os.Args[2:])
	default:
		fmt.Printf("Invalid option: %s\n", os.Args[1])
		os.Exit(1)
	}

	conf := env{pwd, debug}

	serverUrl, err := url.Parse(serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	proxyUrl, err := url.Parse(badProxyAddr)
	if err != nil {
		log.Fatal(err)
	}

	if clientCmd.Parsed() {
		portMapping := [][2]string{}
		for _, i := range strings.Split(pf, ";") {
			if i == "" {
				continue
			}
			j := strings.Split(i, ":")
			portMapping = append(portMapping, [2]string{j[0], j[1]})
		}
		client(portMapping, serverUrl, proxyUrl, conf)
	}

	if serverCmd.Parsed() {
		server(port, conf)
	}
}

func client(portMapping [][2]string, serverUrl, proxyUrl *url.URL, conf env) {
	var wg sync.WaitGroup
	wg.Add(len(portMapping))
	for _, p := range portMapping {
		go func(p [2]string) {
			startClient(p[0], p[1], serverUrl, proxyUrl, conf)
			wg.Done()
		}(p)
	}
	wg.Wait()
}

func server(port string, conf env) {
	startServer(port, conf)
}
