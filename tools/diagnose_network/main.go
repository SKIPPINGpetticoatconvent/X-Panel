package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	fmt.Println("============================================")
	fmt.Println("       X-Panel Network Diagnostic Tool      ")
	fmt.Println("============================================")
	fmt.Printf("OS: %s, Arch: %s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("User ID: %d\n", os.Getuid())

	// Check Root
	if os.Getuid() != 0 {
		fmt.Println("[WARN] Not running as root. Port 80/443 binding may fail.")
	} else {
		fmt.Println("[INFO] Running as root.")
	}

	fmt.Println("\n[1] Checking Port 80 (HTTP) ...")
	checkPort(80)

	fmt.Println("\n[2] Checking Port 443 (HTTPS) ...")
	checkPort(443)

	fmt.Println("\n[3] Checking Firewall (iptables) ...")
	checkFirewall()

	fmt.Println("\n============================================")
}

func checkPort(port int) {
	address := fmt.Sprintf(":%d", port)

	// 1. Dial (Client view)
	fmt.Printf("  -> Actively Connecting to 127.0.0.1%s... ", address)
	conn, err := net.DialTimeout("tcp", "127.0.0.1"+address, 2*time.Second)
	if err == nil {
		fmt.Println("SUCCESS (Port is OPEN and OCCUPIED)")
		conn.Close()
	} else {
		fmt.Printf("FAILED: %v\n", err)
		if isConnectionRefused(err) {
			fmt.Println("     Diagnosis: Connection Refused (Port is FREE but reachable)")
		} else if isTimeout(err) {
			fmt.Println("     Diagnosis: Timeout (Firewall likely DROPPING packets locally)")
		}
	}

	// 2. Bind (Server view)
	fmt.Printf("  -> Attempting to Bind %s... ", address)
	listener, err := net.Listen("tcp", address)
	if err == nil {
		fmt.Println("SUCCESS (Port is FREE and bindable)")
		listener.Close()
	} else {
		fmt.Printf("FAILED: %v\n", err)
		if isPermissionDenied(err) {
			fmt.Println("     Diagnosis: Permission Denied (Root privileges required?)")
		} else if isAddressAlreadyInUse(err) {
			fmt.Println("     Diagnosis: Address Already In Use (Another process is using this port)")
		}
	}
}

func checkFirewall() {
	cmd := exec.Command("iptables", "-L", "-n")
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  -> Failed to list iptables: %v (Permission denied?)\n", err)
		return
	}
	// Simple grep for DROP/REJECT on port 80
	outStr := string(output)
	if strings.Contains(outStr, "dpt:80") {
		fmt.Println("  -> [WARN] Found rules referencing port 80 in iptables.")
	} else {
		fmt.Println("  -> No explicit port 80 rules found in iptables OUTPUT/INPUT (basic check).")
	}
}

func isConnectionRefused(err error) bool {
	for {
		if errors.Is(err, syscall.ECONNREFUSED) {
			return true
		}
		if unwravable, ok := err.(interface{ Unwrap() error }); ok {
			err = unwravable.Unwrap()
			if err == nil {
				break
			}
		} else {
			break
		}
	}
	return false
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return strings.Contains(err.Error(), "i/o timeout")
}

func isPermissionDenied(err error) bool {
	return errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM)
}

func isAddressAlreadyInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}
