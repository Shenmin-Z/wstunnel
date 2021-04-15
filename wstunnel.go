package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
)

const DIAGRAM = `
source process <=> client <=> proxy <=> server <=> destination process
                ^          ^         ^          ^
              conSC      conCP     conPS      conSD
`

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
	clientCmd.StringVar(&pf, "port-forward", "", "port forwarding: local1:remote1;local2:remote2 e.g. 8888:8888;8889:22")
	clientCmd.StringVar(&serverAddr, "server", "", "address where server process is, e.g. http://1.1.1.1:8080")
	clientCmd.StringVar(&badProxyAddr, "proxy", "", "http proxy, e.g. http://user:password@1.1.1.1:8080")
	clientCmd.StringVar(&pwd, "password", "", "password for encrytion (optional)")
	clientCmd.BoolVar(&debug, "debug", false, "show debug info")

	serverCmd.StringVar(&port, "port", "", "port on which the prcess is listening on")
	serverCmd.StringVar(&pwd, "password", "", "password for encrytion (optional)")
	serverCmd.BoolVar(&debug, "debug", false, "show debug info")

	if len(os.Args) < 2 {
		fmt.Println("Client:")
		clientCmd.PrintDefaults()
		fmt.Println("\nServer:")
		serverCmd.PrintDefaults()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "client":
		clientCmd.Parse(os.Args[2:])
	case "server":
		serverCmd.Parse(os.Args[2:])
	default:
		fmt.Printf("Invalid option: %s\n", os.Args[1])
		fmt.Println("Client:")
		clientCmd.PrintDefaults()
		fmt.Println("\nServer:")
		serverCmd.PrintDefaults()
		os.Exit(1)
	}

	conf := env{pwd, debug}

	if clientCmd.Parsed() {
		missingArgs := make([]interface{}, 0)
		if pf == "" {
			missingArgs = append(missingArgs, "port-forward")
		}
		if serverAddr == "" {
			missingArgs = append(missingArgs, "server")
		}
		if badProxyAddr == "" {
			missingArgs = append(missingArgs, "proxy")
		}
		if len(missingArgs) > 0 {
			fmt.Printf("Missing arguments: ")
			fmt.Println(missingArgs...)
			os.Exit(1)
		}

		serverUrl, err := url.Parse(serverAddr)
		if err != nil {
			fmt.Println("Invalid server:", serverAddr)
			os.Exit(1)
		}
		proxyUrl, err := url.Parse(badProxyAddr)
		if err != nil {
			fmt.Println("Invalid proxy:", badProxyAddr)
			os.Exit(1)
		}
		if match, _ := regexp.MatchString(`^(\d+:\d+)?(;\d+:\d+)*$`, pf); !match {
			fmt.Println("Invalid port forward format:", pf)
			os.Exit(1)
		}
		portMapping := [][2]string{}
		for _, i := range strings.Split(pf, ";") {
			if i == "" {
				continue
			}
			j := strings.Split(i, ":")
			portMapping = append(portMapping, [2]string{j[0], j[1]})
		}

		if conf.debug {
			fmt.Println(DIAGRAM)
		}
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

	if serverCmd.Parsed() {
		if port == "" {
			fmt.Println("Missing argument: port")
			os.Exit(1)
		}
		if match, _ := regexp.MatchString(`^\d+$`, port); !match {
			fmt.Println("Invalid port number:", port)
			os.Exit(1)
		}
		if conf.debug {
			fmt.Println(DIAGRAM)
		}
		startServer(port, conf)
	}
}

// -------------------- client -------------------- //
func startClient(localPort, remotePort string, serverUrl, badProxyUrl *url.URL, conf env) {
	activeSC := 0
	activeCP := 0

	clientToProxy := func() (net.Conn, *bufio.Reader, error) {
		conCP, err := net.Dial("tcp", badProxyUrl.Host)
		if err != nil {
			return nil, nil, err
		}
		activeCP++

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
		fmt.Fprint(conCP, writeHead(headers))
		head, bufRd, _ := readHead(conCP)
		if head[0] != "HTTP/1.1 101 Switching Protocols" {
			conCP.Close()
			activeCP--
			return nil, nil, errors.New("Failed to upgrade to websocket.")
		}

		return conCP, bufRd, nil
	}

	sourceToClient := func() {
		l, err := net.Listen("tcp", ":"+localPort)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer l.Close()

		for {
			// client <=> source process
			conSC, err := l.Accept()
			if err != nil {
				fmt.Println(err)
				return
			}
			activeSC++

			go func() {
				// client <=> proxy
				conCP, bufRd, err := clientToProxy()
				if err != nil {
					conSC.Close()
					activeSC--
					return
				}

				debugInfof(conf.debug, "local: %s ---> remote: %s\nClient: SC=%d CP=%d\n", localPort, remotePort, activeSC, activeCP)

				// communicate IV
				iv := genIV()
				fmt.Fprintf(conCP, writeHead([]string{fmt.Sprintf("%x", iv)}))

				wrappedWriter := wrapWriter(conCP, conf.pwd, iv)
				wrappedReader := wrapReader(bufRd, conf.pwd, iv)

				// tell server which port to forward to
				fmt.Fprintf(wrappedWriter, writeHead([]string{fmt.Sprintf("%s", remotePort)}))

				closed := make(chan bool)
				go func() {
					io.Copy(conSC, wrappedReader)
					closed <- true
				}()
				go func() {
					io.Copy(wrappedWriter, conSC)
					closed <- true
				}()
				<-closed
				conSC.Close()
				conCP.Close()
				activeSC--
				activeCP--
				<-closed
				debugInfo(conf.debug, "Client: connection closed.")
			}()
		}
	}

	sourceToClient()
}

// -------------------- server -------------------- //
func startServer(port string, conf env) {
	activePS := 0
	activeSD := 0

	serverToDestination := func(conPS net.Conn, bufRd *bufio.Reader, conf env) {
		ivString, _, _ := readHead(bufRd)
		iv, _ := hex.DecodeString(ivString[0])
		debugInfof(conf.debug, "IV received: %s\n", ivString)

		wrappedWriter := wrapWriter(conPS, conf.pwd, iv)
		wrappedReader := wrapReader(bufRd, conf.pwd, iv)

		content, bufferedWrappedReader, err := readHead(wrappedReader)
		if err != nil {
			conPS.Close()
			activePS--
			return
		}
		remotePort := content[0]
		debugInfof(conf.debug, "Port forwarded to %s\n", remotePort)
		// server <=> destination process
		conSD, err := net.Dial("tcp", "127.0.0.1:"+remotePort)
		if err != nil {
			fmt.Println(err)
			conPS.Close()
			activePS--
			return
		}
		activeSD++
		debugInfof(conf.debug, "Server: PS=%d SD=%d\n", activePS, activeSD)

		closed := make(chan bool)
		go func() {
			io.Copy(wrappedWriter, conSD)
			closed <- true
		}()
		go func() {
			io.Copy(conSD, bufferedWrappedReader)
			closed <- true
		}()
		<-closed
		conPS.Close()
		conSD.Close()
		activePS--
		activeSD--
		<-closed
		debugInfo(conf.debug, "Server: connection closed.")
	}

	proxyToServer := func() {
		l, err := net.Listen("tcp", ":"+port)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer l.Close()

		for {
			// proxy <=> server
			conPS, err := l.Accept()
			if err != nil {
				fmt.Println(err)
				continue
			}
			activePS++

			go func() {
				head, bufRd, _ := readHead(conPS)
				isWstunnel := false
				for _, i := range head {
					if i == "User-Agent: wstunnel-go" {
						isWstunnel = true
						break
					}
				}

				if isWstunnel {
					debugInfo(conf.debug, "Server: received request.")
					fmt.Fprintf(conPS, writeHead([]string{
						"HTTP/1.1 101 Switching Protocols",
						"Upgrade: websocket",
						"Connection: Upgrade",
					}))
					serverToDestination(conPS, bufRd, conf)
				} else {
					fmt.Fprintf(conPS, writeHead([]string{
						"HTTP/1.1 400 Bad Request",
					}))
					conPS.Close()
					activePS--
				}
			}()
		}
	}

	proxyToServer()
}

// -------------------- utils -------------------- //
func writeHead(parts []string) string {
	return strings.Join(parts, "\r\n") + "\r\n\r\n"
}

func readHead(rd io.Reader) ([]string, *bufio.Reader, error) {
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
			fmt.Println("Failed to read new line.")
			return nil, nil, errors.New("Failed to read new line.")
		}
		line = strings.Replace(line, "\r\n", "", 1)
		if line == "" {
			break
		}
		res = append(res, line)
	}
	return res, reader, nil
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
	key := hasher.Sum(nil)[:24]

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
