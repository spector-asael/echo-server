package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

func handleConnection(conn net.Conn, wg *sync.WaitGroup) { // Function to handle connections
	defer wg.Done()
	defer logDisconnection(conn) // Log clients that disconnect
	defer conn.Close()

	logConnection(conn) // Log clients that connect

	err := handleEcho(conn)
	if err != nil {
		logError(conn, err) // Echo server logic
	}
}
func handleEcho(conn net.Conn) error {
	const maxMessageSize int = 1024
	buf := make([]byte, maxMessageSize)

	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		n, err := conn.Read(buf)
		if err != nil {
			return err // Includes EOF
		}

		if n == 1024 {
			conn.Write([]byte("Message cannot be more than 1024 bytes (1024 regular characters).\n"))
			if flushErr := flushExtraInput(conn, buf, maxMessageSize); flushErr != nil {
				return flushErr
			}
			continue
		}

		trimmed := strings.TrimSpace(string(buf[:n]))
		if trimmed == "" {
			continue // ignore empty input
		}

		if _, err := conn.Write([]byte(trimmed + "\n")); err != nil {
			return err
		}
	}
}

func logError(conn net.Conn, err error) {

	clientAddr := conn.RemoteAddr().String()
	logTime := func() string {
		return time.Now().Format(time.RFC3339)
	}

	addr := conn.RemoteAddr().String()
	fmt.Println(err)
	if err == io.EOF {
		fmt.Printf("[%s] Client %s closed the connection (EOF)\n", logTime(), addr)
	} else {
		netErr, ok := err.(net.Error)
		if ok && netErr.Timeout() {
			conn.Write([]byte("Connection timeout. Disconnecting...\n"))
			fmt.Printf("[%s] Timeout: Client %s inactive for 30 seconds\n", logTime(), clientAddr)
			return
		}
	}

}
func logConnection(conn net.Conn) {
	address := conn.RemoteAddr().String()        // Grab address, convert to string
	timestamp := time.Now().Format(time.RFC3339) // Grab current time, format

	fmt.Printf("[%s] New Connection from %s\n", timestamp, address)
}

func logDisconnection(conn net.Conn) {
	address := conn.RemoteAddr().String()
	timestamp := time.Now().Format(time.RFC3339)

	fmt.Printf("[%s] Client %s has disconnected\n", timestamp, address)
}

func parsePortFlag() string {
	port := flag.String("port", "4000", "Port to run the TCP server on.")
	flag.Parse()

	if (*port)[0] != ':' {
		return ":" + *(port)
	}

	return *port
}

func flushExtraInput(conn net.Conn, buf []byte, maxMessageSize int) error {
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	for {
		n, err := conn.Read(buf)
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil // done flushing
		}
		if err != nil {
			return err
		}
		if n < maxMessageSize {
			return nil // no more overflow
		}
	}
}

func main() {

	port := parsePortFlag()
	listener, err := net.Listen("tcp", port) // Set up a tcp connection on port 4000

	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	defer listener.Close() // Close connection at the end of the program

	fmt.Printf("Server listening on %s\n", port)
	for { // Infinite loop to accept multiple connections
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err)
			continue
		}

		wg.Add(1)
		go handleConnection(conn, &wg) // Function to handle connections
	}

	wg.Wait() // Currently unreachable
}
