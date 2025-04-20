package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func worker(conn net.Conn, wg *sync.WaitGroup, workerPool chan struct{}) {

	defer func() {
		<-workerPool // Release slot
		wg.Done()
	}()

	handleConnection(conn)
}

func handleConnection(conn net.Conn) { // Function to handle connections

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

	logger, err := newClientLogger(conn) // Create a clientLogger object that logs messages into a file
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %v", err)
	}
	defer logger.Close()

	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second)) // Time user out after 30 seconds

		n, err := conn.Read(buf)
		if err != nil {
			return err // Includes EOF
		}

		if n == 1024 { // Reject input that is over 1024 bytes
			conn.Write([]byte("Message cannot be more than 1024 bytes (1024 regular characters).\n"))
			if flushErr := flushExtraInput(conn, buf, maxMessageSize); flushErr != nil {
				// remove extra characters from the TCP Stream
				return flushErr
			}
			continue
		}

		trimmed := strings.TrimSpace(string(buf[:n])) // remove spaces at the beginning of the messaage
		if trimmed == "" {
			continue // ignore empty input from user
		}

		if err := logger.Log(trimmed); err != nil { // log message into file
			return fmt.Errorf("failed to log message: %v", err)
		}

		if _, err := conn.Write([]byte(trimmed + "\n")); err != nil { // write message to user
			return err
		}
	}
}

func logError(conn net.Conn, err error) { // logs keep track of errors

	clientAddr := conn.RemoteAddr().String()
	logTime := func() string {
		return time.Now().Format(time.RFC3339)
	}

	addr := conn.RemoteAddr().String()
	fmt.Println(err)
	if err == io.EOF {
		fmt.Printf("[%s] Client %s closed the connection (EOF)\n", logTime(), addr)
		// client closing connection error
	} else {
		netErr, ok := err.(net.Error)
		if ok && netErr.Timeout() {
			conn.Write([]byte("Connection timeout. Disconnecting...\n"))
			fmt.Printf("[%s] Timeout: Client %s inactive for 30 seconds\n", logTime(), clientAddr)
			// timeout error
			return
		}
	}

}
func logConnection(conn net.Conn) {
	address := conn.RemoteAddr().String()        // Grab address, convert to string
	timestamp := time.Now().Format(time.RFC3339) // Grab current time

	fmt.Printf("[%s] New Connection from %s\n", timestamp, address)
}

func logDisconnection(conn net.Conn) {
	address := conn.RemoteAddr().String()
	timestamp := time.Now().Format(time.RFC3339)

	fmt.Printf("[%s] Client %s has disconnected\n", timestamp, address)
}

func parseFlags() (string, int) {
	port := flag.String("port", "4000", "Port to run the TCP server on.")
	workers := flag.String("workers", "5", "Maximum number of concurrent connections.")
	flag.Parse()

	workerCount, err := strconv.Atoi(*workers)
	if err != nil || workerCount < 1 {
		fmt.Printf("Invalid value for -workers: %s. Must be a positive integer.\n", *workers)
		os.Exit(1)
	}

	portStr := *port
	if portStr[0] != ':' {
		portStr = ":" + portStr
	}

	return portStr, workerCount
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

type clientLogger struct { // clientLogger object, so we can attach methods to it
	file *os.File
	ip   string
}

func newClientLogger(conn net.Conn) (*clientLogger, error) { // creates a file to log messages in
	// Use full address (IP:Port), but change ":" to "_"
	rawAddr := conn.RemoteAddr().String()
	safeAddr := strings.ReplaceAll(rawAddr, ":", "_")
	logFilePath := fmt.Sprintf("logs/client_%s.log", safeAddr)

	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	// opens file
	if err != nil {
		return nil, err
	}
	// returns file object to write to
	return &clientLogger{file: file, ip: rawAddr}, nil
}

func (cl *clientLogger) Log(message string) error { // Adds a method to the client Logger object
	timestamp := time.Now().Format(time.RFC3339)
	_, err := cl.file.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, message)) // writing file
	return err
}

func (cl *clientLogger) Close() {
	cl.file.Close()
}
func logRejection(conn net.Conn) {
	address := conn.RemoteAddr().String()
	timestamp := time.Now().Format(time.RFC3339)
	fmt.Printf("[%s] Rejected connection from %s (max connections reached)\n", timestamp, address)
}

func main() {
	port, maxWorkers := parseFlags() // -port flag, default value of 4000
	listener, err := net.Listen("tcp", port)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	workerPool := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	fmt.Printf("Server listening on %s (max %d concurrent clients)\n", port, maxWorkers)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err)
			continue
		}

		select {
		case workerPool <- struct{}{}: // Try to acquire a slot
			wg.Add(1)
			go worker(conn, &wg, workerPool)

		default: // No slots available
			conn.Write([]byte("Server is at max capacity. Try again later.\n"))
			logRejection(conn)
			conn.Close()
		}
	}

	wg.Wait() // unreachable
}
