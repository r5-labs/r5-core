package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"time"

	"github.com/r5-labs/r5-core/gominer/ethash"
)

// RPCRequest represents a JSON-RPC request.
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// RPCResponse represents a JSON-RPC response.
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error"`
	ID      int             `json:"id"`
}

// RPCError represents a JSON-RPC error.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// jsonRPCRequest sends a JSON-RPC request to the node.
func jsonRPCRequest(url, method string, params []interface{}) (json.RawMessage, error) {
	reqBody, err := json.Marshal(RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rpcResp RPCResponse
	if err = json.Unmarshal(body, &rpcResp); err != nil {
		return nil, err
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %d %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

// getWork retrieves work from the node.
func getWork(rpcURL string) (headerHash, seedHash []byte, target *big.Int, err error) {
	result, err := jsonRPCRequest(rpcURL, "eth_getWork", []interface{}{})
	if err != nil {
		return nil, nil, nil, err
	}
	// The result is expected to be an array of three hex strings.
	var work [3]string
	if err := json.Unmarshal(result, &work); err != nil {
		return nil, nil, nil, err
	}

	// Decode header and seed hashes (remove the "0x" prefix).
	headerHash, err = hex.DecodeString(work[0][2:])
	if err != nil {
		return nil, nil, nil, err
	}
	seedHash, err = hex.DecodeString(work[1][2:])
	if err != nil {
		return nil, nil, nil, err
	}

	// Decode the target.
	target = new(big.Int)
	target.SetString(work[2][2:], 16)
	return
}

// submitWork sends a solution back to the node.
func submitWork(rpcURL string, nonce uint64, mixDigest []byte, headerHashStr string) error {
	nonceHex := fmt.Sprintf("0x%016x", nonce)
	mixHex := "0x" + hex.EncodeToString(mixDigest)
	_, err := jsonRPCRequest(rpcURL, "eth_submitWork", []interface{}{nonceHex, headerHashStr, mixHex})
	return err
}

func main() {
	rpcURL := "http://localhost:8545"

	fmt.Println("Requesting work...")
	header, seed, target, err := getWork(rpcURL)
	if err != nil {
		fmt.Printf("Error getting work: %v\n", err)
		return
	}

	fmt.Printf("Work received:\n Header: 0x%s\n Seed: 0x%s\n Target: %s\n",
		hex.EncodeToString(header), hex.EncodeToString(seed), target.String())

	// NEED TO WORK ON THIS TO GET CORRECT BLOCK NUMBER
	var blockNumber uint64 = 1000000

	// Determine the cache and dataset sizes.
	cacheSizeBytes := ethash.CacheSize(blockNumber)
	datasetSizeBytes := ethash.DatasetSize(blockNumber)

	// Allocate a cache as []uint32, since one uint32 is 4 bytes.
	numUint32 := cacheSizeBytes / 4
	cacheUint32 := make([]uint32, numUint32)

	// Convert the epoch value to uint64.
	epoch := blockNumber / ethash.EpochLength

	// Generate the cache using the retrieved seed.
	ethash.GenerateCache(cacheUint32, epoch, seed)
	fmt.Printf("Cache generated: %d uint32 elements\n", len(cacheUint32))
	fmt.Printf("Dataset size: %d bytes\n", datasetSizeBytes)

	// Mining loop: iterate through nonces until a valid solution is found.
	var nonce uint64
	found := false
	for !found {
		// Call HashimotoLight from the ethash package.
		// It expects the dataset size (in bytes), a cache of type []uint32,
		// the header as a byte slice, and the nonce.
		mixDigest, finalHash := ethash.HashimotoLight(datasetSizeBytes, cacheUint32, header, nonce)
		finalNum := new(big.Int).SetBytes(finalHash)

		if finalNum.Cmp(target) < 0 {
			fmt.Printf("Solution found!\n Nonce: %d\n Final hash: 0x%s\n", nonce, hex.EncodeToString(finalHash))
			headerHex := "0x" + hex.EncodeToString(header)
			if err := submitWork(rpcURL, nonce, mixDigest, headerHex); err != nil {
				fmt.Printf("Error submitting work: %v\n", err)
			} else {
				fmt.Println("Solution submitted successfully!")
			}
			found = true
			break
		}
		nonce++
		if nonce%1000000 == 0 {
			fmt.Printf("Tried nonce %d, current hash: 0x%s\n", nonce, hex.EncodeToString(finalHash))
		}
		time.Sleep(1 * time.Millisecond) // NEEDED?
	}
}
