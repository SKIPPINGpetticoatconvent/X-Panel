package main

import (
	"flag"
	"fmt"
	"os"

	"x-ui/database"
	"x-ui/database/model"
)

func main() {
	dbPathPtr := flag.String("db", "/etc/x-ui/x-ui.db", "Path to x-ui.db")
	resetCertPtr := flag.Bool("reset-cert", false, "Reset certificate settings to default (Manual + Empty = HTTP Localhost)")
	sourcePtr := flag.String("source", "", "Set certSource (ip, domain, manual)")
	certPathPtr := flag.String("cert", "", "Set certificate path (for manual webCertFile)")
	keyPathPtr := flag.String("key", "", "Set key path (for manual webKeyFile)")

	flag.Parse()

	if _, err := os.Stat(*dbPathPtr); os.IsNotExist(err) {
		fmt.Printf("Error: Database file not found at %s. Use -db to specify path.\n", *dbPathPtr)
		os.Exit(1)
	}

	err := database.InitDB(*dbPathPtr)
	if err != nil {
		fmt.Printf("Error initializing DB: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		// Close DB logic if needed, but for simple tool we can exit
	}()

	fmt.Println("--- X-Panel Settings Reset Tool ---")

	if *resetCertPtr {
		fmt.Println("Resetting certificate settings to defaults...")
		updateSetting("certSource", "manual")
		updateSetting("webCertFile", "")
		updateSetting("webKeyFile", "")
		updateSetting("ipCertPath", "")
		updateSetting("domainCertPath", "")
		fmt.Println("Done. Panel should now start in HTTP mode on localhost.")
		fmt.Println("Access via SSH Tunnel: ssh -L 13688:127.0.0.1:13688 user@node")
	}

	if *sourcePtr != "" {
		fmt.Printf("Setting certSource to '%s'...\n", *sourcePtr)
		updateSetting("certSource", *sourcePtr)
	}

	if *certPathPtr != "" {
		fmt.Printf("Setting webCertFile to '%s'...\n", *certPathPtr)
		updateSetting("webCertFile", *certPathPtr)
	}

	if *keyPathPtr != "" {
		fmt.Printf("Setting webKeyFile to '%s'...\n", *keyPathPtr)
		updateSetting("webKeyFile", *keyPathPtr)
	}

	fmt.Println("Operation complete.")
}

func updateSetting(key, value string) {
	db := database.GetDB()
	var count int64
	db.Model(&model.Setting{}).Where("key = ?", key).Count(&count)
	if count > 0 {
		err := db.Model(&model.Setting{}).Where("key = ?", key).Update("value", value).Error
		if err != nil {
			fmt.Printf("  [Error] Failed to update %s: %v\n", key, err)
		} else {
			fmt.Printf("  [OK] Updated %s = %s\n", key, value)
		}
	} else {
		setting := &model.Setting{Key: key, Value: value}
		err := db.Create(setting).Error
		if err != nil {
			fmt.Printf("  [Error] Failed to create %s: %v\n", key, err)
		} else {
			fmt.Printf("  [OK] Created %s = %s\n", key, value)
		}
	}
}
