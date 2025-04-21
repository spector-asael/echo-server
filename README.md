# TCP Echo Server w
### Youtube link: https://youtu.be/SJ0A0DTRfv4?si=bzIBAK_i9ILMqvZg 

## How to Run

### 1. Download the Source Code

Clone the repo or manually download `main.go`:

### 2. Run "go run main.go"

Optional flags: 
-workers
-port

Example usage:

go run main.go -workers=50 -port=8080
### 3. Once the server is running, connect to it using the following command:

nc localhost (portnumber)

example usage: nc localhost 4000

### 4. Type a message, and see your message echoed back!

## 5. Commands:
- /echo
- /quit
- /time