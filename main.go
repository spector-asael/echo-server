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

	logger, err := newClientLogger(conn)
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %v", err)
	}
	defer logger.Close()

	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		n, err := conn.Read(buf)
		if err != nil {
			return err
		}

		if n == maxMessageSize {
			conn.Write([]byte("Message cannot be more than 1024 bytes (1024 regular characters).\n"))
			if flushErr := flushExtraInput(conn, buf, maxMessageSize); flushErr != nil {
				return flushErr
			}
			continue
		}

		trimmed := strings.TrimSpace(string(buf[:n]))
		if trimmed == "" {
			conn.Write([]byte("Say something...\n"))
			continue
		}

		// Call the message handler
		if err := handleClientMessage(conn, trimmed); err != nil {
			return err // If the error indicates a client disconnect, return and close the connection
		}

		// Log and echo normal input
		if err := logger.Log(trimmed); err != nil {
			return fmt.Errorf("failed to log message: %v", err)
		}

		if _, err := conn.Write([]byte(trimmed + "\n")); err != nil {
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

func handleClientMessage(conn net.Conn, message string) error {
	// Handle "server personality" messages
	switch strings.ToLower(message) {
	case "hello":
		conn.Write([]byte("Hi there!\n"))
		return nil
	case "bye":
		conn.Write([]byte("Goodbye!\n"))
		return fmt.Errorf("client disconnected")
	case "":
		conn.Write([]byte("Say something...\n"))
		return nil
	}

	// Handle command-based protocol
	if strings.HasPrefix(message, "/") {
		fields := strings.Fields(message)
		cmd := strings.ToLower(fields[0])

		switch cmd {
		case "/time":
			now := time.Now().Format(time.RFC1123)
			conn.Write([]byte("Current time: " + now + "\n"))
		case "/quit":
			conn.Write([]byte("Closing connection...\n"))
			return fmt.Errorf("client disconnected")
		case "/echo":
			if len(fields) > 1 {
				echoMessage := strings.Join(fields[1:], " ")
				conn.Write([]byte(echoMessage + "\n"))
			} else {
				conn.Write([]byte("Usage: /echo <message>\n"))
			}
		default:
			conn.Write([]byte("Unknown command.\n"))
		}
		return nil
	}

	return nil
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
