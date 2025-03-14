// Copyright 2025 R5 Labs
// This file is part of the R5 Core library.
//
// This software is provided "as is", without warranty of any kind,
// express or implied, including but not limited to the warranties
// of merchantability, fitness for a particular purpose and
// noninfringement. In no event shall the authors or copyright
// holders be liable for any claim, damages, or other liability,
// whether in an action of contract, tort or otherwise, arising
// from, out of or in connection with the software or the use or
// other dealings in the software.

//go:build gofuzz
// +build gofuzz

package signify

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"

	fuzz "github.com/google/gofuzz"
	"github.com/jedisct1/go-minisign"
)

func Fuzz(data []byte) int {
	if len(data) < 32 {
		return -1
	}
	tmpFile, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	testSecKey, testPubKey := createKeyPair()
	// Create message
	tmpFile.Write(data)
	if err = tmpFile.Close(); err != nil {
		panic(err)
	}
	// Fuzz comments
	var untrustedComment string
	var trustedComment string
	f := fuzz.NewFromGoFuzz(data)
	f.Fuzz(&untrustedComment)
	f.Fuzz(&trustedComment)
	fmt.Printf("untrusted: %v\n", untrustedComment)
	fmt.Printf("trusted: %v\n", trustedComment)

	err = SignifySignFile(tmpFile.Name(), tmpFile.Name()+".sig", testSecKey, untrustedComment, trustedComment)
	if err != nil {
		panic(err)
	}
	defer os.Remove(tmpFile.Name() + ".sig")

	signify := "signify"
	path := os.Getenv("SIGNIFY")
	if path != "" {
		signify = path
	}

	_, err := exec.LookPath(signify)
	if err != nil {
		panic(err)
	}

	// Write the public key into the file to pass it as
	// an argument to signify-openbsd
	pubKeyFile, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	defer os.Remove(pubKeyFile.Name())
	defer pubKeyFile.Close()
	pubKeyFile.WriteString("untrusted comment: signify public key\n")
	pubKeyFile.WriteString(testPubKey)
	pubKeyFile.WriteString("\n")

	cmd := exec.Command(signify, "-V", "-p", pubKeyFile.Name(), "-x", tmpFile.Name()+".sig", "-m", tmpFile.Name())
	if output, err := cmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("could not verify the file: %v, output: \n%s", err, output))
	}

	// Verify the signature using a golang library
	sig, err := minisign.NewSignatureFromFile(tmpFile.Name() + ".sig")
	if err != nil {
		panic(err)
	}

	pKey, err := minisign.NewPublicKey(testPubKey)
	if err != nil {
		panic(err)
	}

	valid, err := pKey.VerifyFromFile(tmpFile.Name(), sig)
	if err != nil {
		panic(err)
	}
	if !valid {
		panic("invalid signature")
	}
	return 1
}

func getKey(fileS string) (string, error) {
	file, err := os.Open(fileS)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Discard the first line
	scanner.Scan()
	scanner.Scan()
	return scanner.Text(), scanner.Err()
}

func createKeyPair() (string, string) {
	// Create key and put it in correct format
	tmpKey, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tmpKey.Name())
	defer os.Remove(tmpKey.Name() + ".pub")
	defer os.Remove(tmpKey.Name() + ".sec")
	cmd := exec.Command("signify", "-G", "-n", "-p", tmpKey.Name()+".pub", "-s", tmpKey.Name()+".sec")
	if output, err := cmd.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("could not verify the file: %v, output: \n%s", err, output))
	}
	secKey, err := getKey(tmpKey.Name() + ".sec")
	if err != nil {
		panic(err)
	}
	pubKey, err := getKey(tmpKey.Name() + ".pub")
	if err != nil {
		panic(err)
	}
	return secKey, pubKey
}
