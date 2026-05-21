package crypto

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/term"
)

// Wire constants for the container format.
const (
	ModeMachine  byte = 0x00 // key = SHA-256(machine fingerprint)
	ModePassword byte = 0x01 // key = PBKDF2-HMAC-SHA256(password, SHA-256(fp)[:16], 100k)

	NonceSize  = chacha20poly1305.NonceSizeX // 24 bytes (XChaCha20 nonce)
	KeySize    = 32                           // 256-bit
	SaltSize   = 16
	PBKDF2Iter = 100_000
)

var (
	ErrDecryptFailed = errors.New("crypto: decryption failed; the file may be corrupted or the key is wrong")
	ErrInvalidHeader = errors.New("crypto: invalid file header; not a dbexplain encrypted file")
)

// EncryptFile encrypts plaintext and writes the result to dstPath.
//
// Parameters:
//   - plaintext: raw bytes to encrypt (typically the .env file content)
//   - dstPath:   destination file path (typically plaintextFile + ".enc")
//   - machineID: the hex-encoded machine fingerprint from MachineID()
//   - password:  if empty, uses machine-only mode; otherwise PBKDF2 with password
func EncryptFile(plaintext []byte, dstPath, machineID, password string) error {
	var mode byte
	var salt []byte
	var key []byte

	if password == "" {
		mode = ModeMachine
		key = machineKey(machineID)
	} else {
		mode = ModePassword
		salt = make([]byte, SaltSize)
		if _, err := io.ReadFull(rand.Reader, salt); err != nil {
			return fmt.Errorf("crypto: generate salt: %w", err)
		}
		key = passwordKey(password, machineID, salt)
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return fmt.Errorf("crypto: create cipher: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("crypto: generate nonce: %w", err)
	}

	// Encrypt: Seal appends the 16-byte Poly1305 tag to the ciphertext
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)

	// Write output format: [mode][salt?][nonce][ciphertext+tag]
	var buf bytes.Buffer
	buf.WriteByte(mode)
	if mode == ModePassword {
		buf.Write(salt)
	}
	buf.Write(nonce)
	buf.Write(ciphertext)

	return os.WriteFile(dstPath, buf.Bytes(), 0600)
}

// DecryptFile reads an encrypted file and returns the plaintext.
func DecryptFile(srcPath, machineID, password string) ([]byte, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, err
	}
	return DecryptBytes(data, machineID, password)
}

// DecryptBytes decrypts raw encrypted bytes in memory.
func DecryptBytes(data []byte, machineID, password string) ([]byte, error) {
	if len(data) < 1+NonceSize {
		return nil, ErrInvalidHeader
	}

	mode := data[0]
	if mode != ModeMachine && mode != ModePassword {
		return nil, ErrInvalidHeader
	}

	var salt []byte
	var offset int

	if mode == ModePassword {
		if len(data) < 1+SaltSize+NonceSize {
			return nil, ErrInvalidHeader
		}
		salt = data[1 : 1+SaltSize]
		offset = 1 + SaltSize
	} else {
		offset = 1
	}

	nonce := data[offset : offset+NonceSize]
	ciphertext := data[offset+NonceSize:]

	// Derive key
	var key []byte
	if mode == ModeMachine {
		if password != "" {
			return nil, fmt.Errorf("crypto: file encrypted in machine-only mode; do not supply a password")
		}
		key = machineKey(machineID)
	} else {
		if password == "" {
			return nil, fmt.Errorf("crypto: file encrypted with a password; supply password via APP_ENCRYPTION_KEY")
		}
		key = passwordKey(password, machineID, salt)
	}

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: create cipher: %w", err)
	}

	// Open decrypts and verifies the authentication tag
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptFailed, err)
	}

	return plaintext, nil
}

// machineKey converts the hex machine fingerprint to a 32-byte key via SHA-256.
func machineKey(fp string) []byte {
	h := sha256.Sum256([]byte(fp))
	return h[:]
}

// passwordKey derives a 32-byte key from a password + machine fingerprint + salt.
// The fingerprint ensures the key is still hardware-bound even with a password.
func passwordKey(password, fp string, salt []byte) []byte {
	// Use the first 16 bytes of SHA-256(fp) as the PBKDF2 salt component.
	// This binds the derived key to the specific machine.
	fpHash := sha256.Sum256([]byte(fp))
	combined := make([]byte, len(salt)+16)
	copy(combined, salt)
	copy(combined[len(salt):], fpHash[:16])
	return pbkdf2.Key([]byte(password), combined, PBKDF2Iter, KeySize, sha256.New)
}

// ReadPassword prompts the user and reads a password from stdin without echo.
func ReadPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("crypto: read password: %w", err)
	}
	return string(password), nil
}
