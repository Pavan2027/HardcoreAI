package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	platformio "hardcore-debug/PlatformIO"
	QEMU "hardcore-debug/QEMU"
	debug "hardcore-debug/debug"
)

var dbg *debug.GDBDebugger

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","debugger_connected":%t}`, dbg != nil)
}

func requireDebugger(w http.ResponseWriter) bool {
	if dbg == nil {
		http.Error(w, "Debugger is not connected", http.StatusConflict)
		return false
	}
	return true
}

func ConnectHandler(
	w http.ResponseWriter,
	r *http.Request,
) {

	if dbg != nil {
		dbg.Disconnect()
	}

	debugger := &debug.GDBDebugger{}

	err := debugger.Connect()

	if err != nil {

		http.Error(
			w,
			err.Error(),
			500,
		)

		return
	}

	dbg = debugger

	fmt.Fprintf(
		w,
		"Debugger Connected",
	)
}
func RegistersHandler(w http.ResponseWriter, r *http.Request) {
	if !requireDebugger(w) {
		return
	}

	regs, err := dbg.ReadRegisters()

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	fmt.Fprintf(w,
		"PC: 0x%08X\nSP: 0x%08X\nLR: 0x%08X\nXPSR: 0x%08X\n\nR0: 0x%08X\nR1: 0x%08X\nR2: 0x%08X\nR3: 0x%08X\nR4: 0x%08X\nR5: 0x%08X\nR6: 0x%08X\nR7: 0x%08X\nR8: 0x%08X\nR9: 0x%08X\nR10: 0x%08X\nR11: 0x%08X\nR12: 0x%08X",
		regs.PC, regs.SP, regs.LR, regs.XPSR,
		regs.R0, regs.R1, regs.R2, regs.R3,
		regs.R4, regs.R5, regs.R6, regs.R7,
		regs.R8, regs.R9, regs.R10, regs.R11, regs.R12,
	)
}

func HaltHandler(w http.ResponseWriter, r *http.Request) {
	if !requireDebugger(w) {
		return
	}

	err := dbg.Halt()

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Write([]byte("CPU Halted"))
}

func ContinueHandler(w http.ResponseWriter, r *http.Request) {
	if !requireDebugger(w) {
		return
	}

	err := dbg.Continue()

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Write([]byte("CPU Running"))
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			return
		}

		next.ServeHTTP(w, r)
	})
}

func EmulateHandler(w http.ResponseWriter, r *http.Request) {

	output, err := QEMU.RunQEMU(
		"./Blinky/.pio/build/genericSTM32F405RG/firmware.elf",
	)

	if err != nil {
		w.Write([]byte(err.Error() + "\n" + output))
		return
	}

	w.Write([]byte(output))
}

func QemuStreamHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	ch := QEMU.Subscribe()
	defer QEMU.Unsubscribe(ch)

	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
func StepHandler(
	w http.ResponseWriter,
	r *http.Request,
) {

	defer func() {

		if rec := recover(); rec != nil {

			http.Error(
				w,
				fmt.Sprintf(
					"panic: %v",
					rec,
				),
				500,
			)
		}
	}()

	if !requireDebugger(w) {
		return
	}

	err := dbg.Step()

	if err != nil {

		http.Error(
			w,
			err.Error(),
			500,
		)

		return
	}

	fmt.Fprintf(
		w,
		"CPU Stepped",
	)
}
func main() {

	http.HandleFunc("/health", HealthHandler)
	http.HandleFunc("/platformio/build", platformio.BuildHandler)
	http.HandleFunc("/platformio/flash", platformio.FlashHandler)
	http.HandleFunc("/qemu/run", EmulateHandler)
	http.HandleFunc("/qemu/stream", QemuStreamHandler)
	http.HandleFunc("/debug/connect", ConnectHandler)
	http.HandleFunc("/debug/registers", RegistersHandler)
	http.HandleFunc("/debug/halt", HaltHandler)
	http.HandleFunc("/debug/continue", ContinueHandler)
	http.HandleFunc(
		"/debug/step",
		StepHandler,
	)

	host := envOrDefault("EMULATOR_HOST", "127.0.0.1")
	port := envOrDefault("EMULATOR_PORT", "62019")
	addr := net.JoinHostPort(host, port)

	fmt.Printf("Server running on http://%s\n", addr)

	err := http.ListenAndServe(
		addr,
		enableCORS(http.DefaultServeMux),
	)

	if err != nil {
		panic(err)
	}
}
