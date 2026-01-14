package crypto_test

import (
	"log"
	"testing"

	"x-ui/util/crypto"
)

func TestHashPasswordAsBcrypt(t *testing.T) {
	password := "123456"
	hash, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		t.Fatalf("HashPasswordAsBcrypt failed: %v", err)
	}

	if hash == "" {
		t.Error("HashPasswordAsBcrypt returned empty string")
	}

	// Bcrypt should generate different hashes for same password due to salt
	hash2, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		t.Fatalf("HashPasswordAsBcrypt failed: %v", err)
	}

	if hash == hash2 {
		t.Error("HashPasswordAsBcrypt returned same hash for same password (salt not working?)")
	}

	log.Printf("Hash: %s", hash)
}

func TestCheckPasswordHash(t *testing.T) {
	password := "123456"
	hash, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		t.Fatalf("HashPasswordAsBcrypt failed: %v", err)
	}

	if !crypto.CheckPasswordHash(hash, password) {
		t.Error("CheckPasswordHash returned false for correct password")
	}

	if crypto.CheckPasswordHash(hash, "wrongpassword") {
		t.Error("CheckPasswordHash returned true for wrong password")
	}
}
