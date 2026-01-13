package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"x-ui/database"
	"x-ui/database/model"
)

// Simplified version of SettingService for debugging
type SettingService struct{}

func (s *SettingService) getSetting(key string) (string, error) {
	db := database.GetDB()
	var setting model.Setting
	err := db.Model(&model.Setting{}).Where("key = ?", key).First(&setting).Error
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func main() {
	fmt.Println("============================================")
	fmt.Println("       X-Panel Startup Diagnostic Tool      ")
	fmt.Println("============================================")
	fmt.Printf("Time: %s\n", time.Now().Format(time.RFC3339))
	fmt.Printf("OS: %s, Arch: %s\n", runtime.GOOS, runtime.GOARCH)

	// 1. Initialize DB
	dbPath := "/etc/x-ui/x-ui.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}
	fmt.Printf("[1] Database Path: %s\n", dbPath)

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Printf("    [ERROR] Database file not found!\n")
		return
	}

	err := database.InitDB(dbPath)
	if err != nil {
		fmt.Printf("    [ERROR] Failed to init DB: %v\n", err)
		return
	}
	fmt.Println("    [OK] Database connection successful.")

	s := &SettingService{}

	// 2. Check Cert Source
	certSource, _ := s.getSetting("certSource")
	fmt.Printf("\n[2] Certificate Source (certSource): '%s'\n", certSource)
	if certSource == "" {
		fmt.Println("    [WARN] certSource is empty! Defaulting to 'manual' effectively.")
	}

	// 3. Check Effective Paths
	var certPath, keyPath string

	switch certSource {
	case "ip":
		basePath, _ := s.getSetting("ipCertPath")
		fmt.Printf("    -> Mode: IP\n")
		fmt.Printf("    -> ipCertPath Setting: '%s'\n", basePath)
		if basePath != "" {
			certPath = basePath + ".crt"
			keyPath = basePath + ".key"
		}
	case "domain":
		basePath, _ := s.getSetting("domainCertPath")
		fmt.Printf("    -> Mode: Domain\n")
		fmt.Printf("    -> domainCertPath Setting: '%s'\n", basePath)
		if basePath != "" {
			certPath = basePath + ".crt"
			keyPath = basePath + ".key"
		}
	default: // manual or others
		c, _ := s.getSetting("webCertFile")
		k, _ := s.getSetting("webKeyFile")
		fmt.Printf("    -> Mode: Manual (default)\n")
		fmt.Printf("    -> webCertFile: '%s'\n", c)
		fmt.Printf("    -> webKeyFile : '%s'\n", k)
		certPath = c
		keyPath = k
	}

	fmt.Printf("\n[3] Resolved Certificate Paths:\n")
	fmt.Printf("    Cert: '%s'\n", certPath)
	fmt.Printf("    Key : '%s'\n", keyPath)

	// 4. File Existence Check
	missing := false
	if certPath != "" {
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			fmt.Printf("    [ERROR] Cert file does NOT exist on disk!\n")
			missing = true
		} else {
			fmt.Printf("    [OK] Cert file exists.\n")
		}
	} else {
		fmt.Printf("    [WARN] Cert path is empty.\n")
		missing = true
	}

	if keyPath != "" {
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			fmt.Printf("    [ERROR] Key file does NOT exist on disk!\n")
			missing = true
		} else {
			fmt.Printf("    [OK] Key file exists.\n")
		}
	} else {
		fmt.Printf("    [WARN] Key path is empty.\n")
		missing = true
	}

	// 5. Startup Logic Simulation
	fmt.Printf("\n[4] Startup Logic Simulation:\n")
	if !missing && certPath != "" && keyPath != "" {
		fmt.Println("    -> HTTPS ENABLED. Validation passed.")
		listen, _ := s.getSetting("webListen")
		port, _ := s.getSetting("webPort")
		fmt.Printf("    -> Listening on: %s:%s (HTTPS)\n", listen, port)
	} else {
		fmt.Println("    -> FALLBACK TO HTTP/LOCALHOST (Security Feature).")
		fmt.Println("    -> Because certificates are missing or invalid, X-Panel forces 127.0.0.1 binding.")
		fmt.Println("    [!] This explains why you cannot access the panel remotely.")
	}

	fmt.Println("============================================")
}
