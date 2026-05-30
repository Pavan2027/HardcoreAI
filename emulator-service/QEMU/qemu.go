package qemu

import (
	"bufio"
	"net"
	"os/exec"
	"sync"
	"time"
)

var (
	qemuCmd    *exec.Cmd
	clients    = make(map[chan string]bool)
	clientsMtx sync.Mutex
)

func Subscribe() chan string {
	ch := make(chan string, 100)
	clientsMtx.Lock()
	clients[ch] = true
	clientsMtx.Unlock()
	return ch
}

func Unsubscribe(ch chan string) {
	clientsMtx.Lock()
	delete(clients, ch)
	clientsMtx.Unlock()
	close(ch)
}

func broadcast(msg string) {
	clientsMtx.Lock()
	defer clientsMtx.Unlock()
	for ch := range clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func RunQEMU(firmwarePath string) (string, error) {

	if qemuCmd != nil && qemuCmd.Process != nil {
		qemuCmd.Process.Kill()
		qemuCmd.Wait()
	}

	// Force kill any orphaned QEMU processes to free up ports 3333 and 4444
	exec.Command("taskkill", "/F", "/IM", "qemu-system-arm.exe").Run()

	qemuCmd = exec.Command(
		"qemu-system-arm",
		"-M", "olimex-stm32-h405",
		"-kernel", firmwarePath,
		"-gdb", "tcp::3333",
		"-display", "none",
		"-serial", "null", // USART1 -> null
		"-serial", "tcp:127.0.0.1:4444,server,nowait", // USART2 -> port 4444
	)

	err := qemuCmd.Start()

	if err != nil {
		return "", err
	}

	go func() {
		var conn net.Conn
		var err error
		// Retry connecting to QEMU's serial TCP port for up to 5 seconds
		for i := 0; i < 50; i++ {
			conn, err = net.Dial("tcp", "127.0.0.1:4444")
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if conn != nil {
			defer conn.Close()
			scanner := bufio.NewScanner(conn)
			for scanner.Scan() {
				broadcast(scanner.Text())
			}
		}
	}()

	return "QEMU Started", nil
}
