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

package abi

import (
	"fmt"
	"strings"

	"github.com/r5-labs/r5-core/client/crypto"
)

// FunctionType represents different types of functions a contract might have.
type FunctionType int

const (
	// Constructor represents the constructor of the contract.
	// The constructor function is called while deploying a contract.
	Constructor FunctionType = iota
	// Fallback represents the fallback function.
	// This function is executed if no other function matches the given function
	// signature and no receive function is specified.
	Fallback
	// Receive represents the receive function.
	// This function is executed on plain Ether transfers.
	Receive
	// Function represents a normal function.
	Function
)

// Method represents a callable given a `Name` and whether the method is a constant.
// If the method is `Const` no transaction needs to be created for this
// particular Method call. It can easily be simulated using a local VM.
// For example a `Balance()` method only needs to retrieve something
// from the storage and therefore requires no Tx to be sent to the
// network. A method such as `Transact` does require a Tx and thus will
// be flagged `false`.
// Input specifies the required input parameters for this gives method.
type Method struct {
	// Name is the method name used for internal representation. It's derived from
	// the raw name and a suffix will be added in the case of a function overload.
	//
	// e.g.
	// These are two functions that have the same name:
	// * foo(int,int)
	// * foo(uint,uint)
	// The method name of the first one will be resolved as foo while the second one
	// will be resolved as foo0.
	Name    string
	RawName string // RawName is the raw method name parsed from ABI

	// Type indicates whether the method is a
	// special fallback introduced in solidity v0.6.0
	Type FunctionType

	// StateMutability indicates the mutability state of method,
	// the default value is nonpayable. It can be empty if the abi
	// is generated by legacy compiler.
	StateMutability string

	// Legacy indicators generated by compiler before v0.6.0
	Constant bool
	Payable  bool

	Inputs  Arguments
	Outputs Arguments
	str     string
	// Sig returns the methods string signature according to the ABI spec.
	// e.g.		function foo(uint32 a, int b) = "foo(uint32,int256)"
	// Please note that "int" is substitute for its canonical representation "int256"
	Sig string
	// ID returns the canonical representation of the method's signature used by the
	// abi definition to identify method names and types.
	ID []byte
}

// NewMethod creates a new Method.
// A method should always be created using NewMethod.
// It also precomputes the sig representation and the string representation
// of the method.
func NewMethod(name string, rawName string, funType FunctionType, mutability string, isConst, isPayable bool, inputs Arguments, outputs Arguments) Method {
	var (
		types       = make([]string, len(inputs))
		inputNames  = make([]string, len(inputs))
		outputNames = make([]string, len(outputs))
	)
	for i, input := range inputs {
		inputNames[i] = fmt.Sprintf("%v %v", input.Type, input.Name)
		types[i] = input.Type.String()
	}
	for i, output := range outputs {
		outputNames[i] = output.Type.String()
		if len(output.Name) > 0 {
			outputNames[i] += fmt.Sprintf(" %v", output.Name)
		}
	}
	// calculate the signature and method id. Note only function
	// has meaningful signature and id.
	var (
		sig string
		id  []byte
	)
	if funType == Function {
		sig = fmt.Sprintf("%v(%v)", rawName, strings.Join(types, ","))
		id = crypto.Keccak256([]byte(sig))[:4]
	}
	// Extract meaningful state mutability of solidity method.
	// If it's default value, never print it.
	state := mutability
	if state == "nonpayable" {
		state = ""
	}
	if state != "" {
		state = state + " "
	}
	identity := fmt.Sprintf("function %v", rawName)
	if funType == Fallback {
		identity = "fallback"
	} else if funType == Receive {
		identity = "receive"
	} else if funType == Constructor {
		identity = "constructor"
	}
	str := fmt.Sprintf("%v(%v) %sreturns(%v)", identity, strings.Join(inputNames, ", "), state, strings.Join(outputNames, ", "))

	return Method{
		Name:            name,
		RawName:         rawName,
		Type:            funType,
		StateMutability: mutability,
		Constant:        isConst,
		Payable:         isPayable,
		Inputs:          inputs,
		Outputs:         outputs,
		str:             str,
		Sig:             sig,
		ID:              id,
	}
}

func (method Method) String() string {
	return method.str
}

// IsConstant returns the indicator whether the method is read-only.
func (method Method) IsConstant() bool {
	return method.StateMutability == "view" || method.StateMutability == "pure" || method.Constant
}

// IsPayable returns the indicator whether the method can process
// plain ether transfers.
func (method Method) IsPayable() bool {
	return method.StateMutability == "payable" || method.Payable
}
