// Copyright 2025 R5
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

package keystore

import (
	"os"

	"github.com/r5-codebase/r5-core/accounts/keystore"
)

func Fuzz(input []byte) int {
	ks := keystore.NewKeyStore("/tmp/ks", keystore.LightScryptN, keystore.LightScryptP)

	a, err := ks.NewAccount(string(input))
	if err != nil {
		panic(err)
	}
	if err := ks.Unlock(a, string(input)); err != nil {
		panic(err)
	}
	os.Remove(a.URL.Path)
	return 1
}
