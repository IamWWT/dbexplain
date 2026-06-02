// Package encrypt handles the "encrypt" subcommand for .env file encryption.
package encrypt

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/IamWWT/dbexplain/internal/crypto"
	"github.com/IamWWT/dbexplain/internal/config"
)

// Handle processes the encrypt subcommand.
func Handle(args []string) {
	showHelp := false
	usePassword := false
	outputFile := ""
	inputFile := ""

	i := 0
	for i < len(args) {
		switch args[i] {
		case "-h", "--help":
			showHelp = true
		case "-password", "--password":
			usePassword = true
		case "-o", "--output":
			if i+1 < len(args) {
				i++
				outputFile = args[i]
			} else {
				log.Fatal("crypto: -o requires a file path argument")
			}
		default:
			if !strings.HasPrefix(args[i], "-") {
				inputFile = args[i]
			} else {
				log.Fatalf("crypto: unknown flag: %s", args[i])
			}
		}
		i++
	}

	if showHelp {
		fmt.Fprint(os.Stderr, "Usage: dbexplain encrypt [flags] [<file>]\n\n"+
			"Encrypt a .env configuration file using machine fingerprint.\n"+
			"The encrypted file can only be decrypted on the same machine.\n\n"+
			"Flags:\n"+
			"  -password, --password   Prompt for a password (PBKDF2 + machine fingerprint)\n"+
			"  -o, --output <file>     Output file path (default: <input>.enc)\n"+
			"  -h, --help              Show this help\n\n"+
			"Examples:\n"+
			"  dbexplain encrypt                        # uses .env.dbexplain in CWD or config dir\n"+
			"  dbexplain encrypt .env.dbexplain          # explicit input file\n"+
			"  dbexplain encrypt --password              # password + machine fingerprint\n"+
			"  dbexplain encrypt -o config.enc .env\n")
		return
	}

	if inputFile == "" {
		inputFile = config.FindConfigFile()
		if inputFile == "" {
			log.Fatal("crypto: no config file found. Specify a file path or create .env.dbexplain")
		}
	}

	plaintext, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("crypto: read input file %s: %v", inputFile, err)
	}

	plaintext = bytes.TrimPrefix(plaintext, []byte{0xEF, 0xBB, 0xBF})

	if len(plaintext) > 0 && (plaintext[0] == crypto.ModeMachine || plaintext[0] == crypto.ModePassword) {
		fmt.Fprintf(os.Stderr, "Warning: %s appears to be already encrypted. Proceeding anyway.\n", inputFile)
	}

	machineID, err := crypto.MachineID()
	if err != nil {
		log.Fatalf("crypto: compute machine fingerprint: %v", err)
	}

	var password string
	if usePassword {
		pwd, err := crypto.ReadPassword("Enter encryption password: ")
		if err != nil {
			log.Fatalf("crypto: %v", err)
		}
		pwd2, err := crypto.ReadPassword("Confirm password: ")
		if err != nil {
			log.Fatalf("crypto: %v", err)
		}
		if pwd != pwd2 {
			log.Fatal("crypto: passwords do not match")
		}
		password = pwd
	}

	var dstPath string
	if outputFile != "" {
		dstPath = outputFile
	} else {
		dstPath = inputFile + ".enc"
	}

	if err := crypto.EncryptFile(plaintext, dstPath, machineID, password); err != nil {
		log.Fatalf("crypto: encrypt: %v", err)
	}

	if password != "" {
		fmt.Fprintf(os.Stderr, "Encrypted with machine fingerprint + password: %s\n", dstPath)
		fmt.Fprintf(os.Stderr, "Save your password to %s.encryption_key before running dbexplain -env.\n", config.ConfigDirDisplay())
	} else {
		fmt.Fprintf(os.Stderr, "Encrypted with machine fingerprint: %s\n", dstPath)
		fmt.Fprintf(os.Stderr, "File can only be decrypted on this machine.\n")
	}
	fmt.Fprintf(os.Stderr, "Place this file in %s (or CWD) and run: dbexplain -env\n", config.ConfigDirDisplay())
}
