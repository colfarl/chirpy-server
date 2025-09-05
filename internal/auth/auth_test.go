package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEncryptDecrypt(t *testing.T) {
	passwords := []string{
		"password1",
		"Lajda;nva;s",
		"huh",
		"PaSwOrd123!",
	}

	for _, password := range passwords {

		hash, err := HashPassword(password)
		if err != nil {
			t.Errorf(`Password %s, could not be hashed`, hash)
		}
		
		err = CheckPasswordHash(password, hash)
		if err != nil {
			t.Errorf(`Password %s, has mismatched hash: %v`, hash, err)
		}

	}
}

func TestMakeValidateJWT(t * testing.T) {
	

	//Test Basic
	id := uuid.New()
	tok, err := MakeJWT(id, "super-secret", 5*time.Second)
	if err != nil {
		t.Errorf(`"%v" could not be made`, id)
	}

	got, err := ValidateJWT(tok, "super-secret")
	if err != nil {	
		t.Errorf(`"%v" could not be validated`, tok)
	}

	if got != id {
		t.Errorf(`"%v" does not equal "%v" `, got, id)
	}


	// Test Time Out
	id = uuid.New()
	tok, err = MakeJWT(id, "super-secret", 1*time.Second)
	if err != nil {
		t.Errorf(`"%v" could not be made`, id)
	}

	time.Sleep(2 * time.Second)
	got, err = ValidateJWT(tok, "super-secret")
	if err == nil {	
		t.Errorf(`"%v" should have times out`, tok)
	}

	if got == id {
		t.Errorf(`"%v" should not equal "%v" `, got, id)
	}

	// Test Invalid
	id = uuid.New()
	tok, err = MakeJWT(id, "super-secret", 5*time.Second)
	if err != nil {
		t.Errorf(`"%v" could not be made`, id)
	}

	got, err = ValidateJWT(tok, "diff-secret")
	if err == nil {	
		t.Errorf(`"%v" should have been decrypted`, tok)
	}

	if got == id {
		t.Errorf(`"%v" does not equal "%v" `, got, id)
	}
}
