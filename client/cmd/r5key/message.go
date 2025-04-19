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

package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/r5-labs/r5-core/client/accounts"
	"github.com/r5-labs/r5-core/client/accounts/keystore"
	"github.com/r5-labs/r5-core/client/cmd/utils"
	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/urfave/cli/v2"
)

type outputSign struct {
	Signature string
}

var msgfileFlag = &cli.StringFlag{
	Name:  "msgfile",
	Usage: "file containing the message to sign/verify",
}

var commandSignMessage = &cli.Command{
	Name:      "signmessage",
	Usage:     "sign a message",
	ArgsUsage: "<keyfile> <message>",
	Description: `
Sign the message with a keyfile.

To sign a message contained in a file, use the --msgfile flag.
`,
	Flags: []cli.Flag{
		passphraseFlag,
		jsonFlag,
		msgfileFlag,
	},
	Action: func(ctx *cli.Context) error {
		message := getMessage(ctx, 1)

		// Load the keyfile.
		keyfilepath := ctx.Args().First()
		keyjson, err := os.ReadFile(keyfilepath)
		if err != nil {
			utils.Fatalf("Failed to read the keyfile at '%s': %v", keyfilepath, err)
		}

		// Decrypt key with passphrase.
		passphrase := getPassphrase(ctx, false)
		key, err := keystore.DecryptKey(keyjson, passphrase)
		if err != nil {
			utils.Fatalf("Error decrypting key: %v", err)
		}

		signature, err := crypto.Sign(accounts.TextHash(message), key.PrivateKey)
		if err != nil {
			utils.Fatalf("Failed to sign message: %v", err)
		}
		out := outputSign{Signature: hex.EncodeToString(signature)}
		if ctx.Bool(jsonFlag.Name) {
			mustPrintJSON(out)
		} else {
			fmt.Println("Signature:", out.Signature)
		}
		return nil
	},
}

type outputVerify struct {
	Success            bool
	RecoveredAddress   string
	RecoveredPublicKey string
}

var commandVerifyMessage = &cli.Command{
	Name:      "verifymessage",
	Usage:     "verify the signature of a signed message",
	ArgsUsage: "<address> <signature> <message>",
	Description: `
Verify the signature of the message.
It is possible to refer to a file containing the message.`,
	Flags: []cli.Flag{
		jsonFlag,
		msgfileFlag,
	},
	Action: func(ctx *cli.Context) error {
		addressStr := ctx.Args().First()
		signatureHex := ctx.Args().Get(1)
		message := getMessage(ctx, 2)

		if !common.IsHexAddress(addressStr) {
			utils.Fatalf("Invalid address: %s", addressStr)
		}
		address := common.HexToAddress(addressStr)
		signature, err := hex.DecodeString(signatureHex)
		if err != nil {
			utils.Fatalf("Signature encoding is not hexadecimal: %v", err)
		}

		recoveredPubkey, err := crypto.SigToPub(accounts.TextHash(message), signature)
		if err != nil || recoveredPubkey == nil {
			utils.Fatalf("Signature verification failed: %v", err)
		}
		recoveredPubkeyBytes := crypto.FromECDSAPub(recoveredPubkey)
		recoveredAddress := crypto.PubkeyToAddress(*recoveredPubkey)
		success := address == recoveredAddress

		out := outputVerify{
			Success:            success,
			RecoveredPublicKey: hex.EncodeToString(recoveredPubkeyBytes),
			RecoveredAddress:   recoveredAddress.Hex(),
		}
		if ctx.Bool(jsonFlag.Name) {
			mustPrintJSON(out)
		} else {
			if out.Success {
				fmt.Println("Signature verification successful!")
			} else {
				fmt.Println("Signature verification failed!")
			}
			fmt.Println("Recovered public key:", out.RecoveredPublicKey)
			fmt.Println("Recovered address:", out.RecoveredAddress)
		}
		return nil
	},
}

func getMessage(ctx *cli.Context, msgarg int) []byte {
	if file := ctx.String(msgfileFlag.Name); file != "" {
		if ctx.NArg() > msgarg {
			utils.Fatalf("Can't use --msgfile and message argument at the same time.")
		}
		msg, err := os.ReadFile(file)
		if err != nil {
			utils.Fatalf("Can't read message file: %v", err)
		}
		return msg
	} else if ctx.NArg() == msgarg+1 {
		return []byte(ctx.Args().Get(msgarg))
	}
	utils.Fatalf("Invalid number of arguments: want %d, got %d", msgarg+1, ctx.NArg())
	return nil
}
