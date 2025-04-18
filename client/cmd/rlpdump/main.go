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

// rlpdump is a pretty-printer for RLP data.
package main

import (
	"bufio"
	"bytes"
	"container/list"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/rlp"
)

var (
	hexMode     = flag.String("hex", "", "dump given hex data")
	reverseMode = flag.Bool("reverse", false, "convert ASCII to rlp")
	noASCII     = flag.Bool("noascii", false, "don't print ASCII strings readably")
	single      = flag.Bool("single", false, "print only the first element, discard the rest")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage:", os.Args[0], "[-noascii] [-hex <data>][-reverse] [filename]")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, `
Dumps RLP data from the given file in readable form.
If the filename is omitted, data is read from stdin.`)
	}
}

func main() {
	flag.Parse()

	var r io.Reader
	switch {
	case *hexMode != "":
		data, err := hex.DecodeString(strings.TrimPrefix(*hexMode, "0x"))
		if err != nil {
			die(err)
		}
		r = bytes.NewReader(data)

	case flag.NArg() == 0:
		r = os.Stdin

	case flag.NArg() == 1:
		fd, err := os.Open(flag.Arg(0))
		if err != nil {
			die(err)
		}
		defer fd.Close()
		r = fd

	default:
		fmt.Fprintln(os.Stderr, "Error: too many arguments")
		flag.Usage()
		os.Exit(2)
	}
	out := os.Stdout
	if *reverseMode {
		data, err := textToRlp(r)
		if err != nil {
			die(err)
		}
		fmt.Printf("%#x\n", data)
		return
	} else {
		err := rlpToText(r, out)
		if err != nil {
			die(err)
		}
	}
}

func rlpToText(r io.Reader, out io.Writer) error {
	s := rlp.NewStream(r, 0)
	for {
		if err := dump(s, 0, out); err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
		fmt.Fprintln(out)
		if *single {
			break
		}
	}
	return nil
}

func dump(s *rlp.Stream, depth int, out io.Writer) error {
	kind, size, err := s.Kind()
	if err != nil {
		return err
	}
	switch kind {
	case rlp.Byte, rlp.String:
		str, err := s.Bytes()
		if err != nil {
			return err
		}
		if len(str) == 0 || !*noASCII && isASCII(str) {
			fmt.Fprintf(out, "%s%q", ws(depth), str)
		} else {
			fmt.Fprintf(out, "%s%x", ws(depth), str)
		}
	case rlp.List:
		s.List()
		defer s.ListEnd()
		if size == 0 {
			fmt.Fprintf(out, ws(depth)+"[]")
		} else {
			fmt.Fprintln(out, ws(depth)+"[")
			for i := 0; ; i++ {
				if i > 0 {
					fmt.Fprint(out, ",\n")
				}
				if err := dump(s, depth+1, out); err == rlp.EOL {
					break
				} else if err != nil {
					return err
				}
			}
			fmt.Fprint(out, ws(depth)+"]")
		}
	}
	return nil
}

func isASCII(b []byte) bool {
	for _, c := range b {
		if c < 32 || c > 126 {
			return false
		}
	}
	return true
}

func ws(n int) string {
	return strings.Repeat("  ", n)
}

func die(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}

// textToRlp converts text into RLP (best effort).
func textToRlp(r io.Reader) ([]byte, error) {
	// We're expecting the input to be well-formed, meaning that
	// - each element is on a separate line
	// - each line is either an (element OR a list start/end) + comma
	// - an element is either hex-encoded bytes OR a quoted string
	var (
		scanner = bufio.NewScanner(r)
		obj     []interface{}
		stack   = list.New()
	)
	for scanner.Scan() {
		t := strings.TrimSpace(scanner.Text())
		if len(t) == 0 {
			continue
		}
		switch t {
		case "[": // list start
			stack.PushFront(obj)
			obj = make([]interface{}, 0)
		case "]", "],": // list end
			parent := stack.Remove(stack.Front()).([]interface{})
			obj = append(parent, obj)
		case "[],": // empty list
			obj = append(obj, make([]interface{}, 0))
		default: // element
			data := []byte(t)[:len(t)-1] // cut off comma
			if data[0] == '"' {          // ascii string
				data = []byte(t)[1 : len(data)-1]
			} else { // hex data
				data = common.FromHex(string(data))
			}
			obj = append(obj, data)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	data, err := rlp.EncodeToBytes(obj[0])
	return data, err
}
