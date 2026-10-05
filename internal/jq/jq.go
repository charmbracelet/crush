// Package jq runs the jq programs a plugin declares. Both of the plugin's
// extension points — rewriting a provider's traffic and reading its model
// catalog — are jq, and they speak one dialect so a program author learns it
// once. The dialect is jq plus the functions below.
package jq

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/itchyny/gojq"
)

// Program is a compiled jq program, reusable across inputs.
type Program struct {
	code *gojq.Code
}

// Compile parses and compiles a program with the plugin dialect available to
// it. Compiling once and running many times keeps a per-request program off
// the hot path.
func Compile(source string) (*Program, error) {
	query, err := gojq.Parse(source)
	if err != nil {
		return nil, err
	}
	code, err := gojq.Compile(query,
		gojq.WithFunction("sha256hex", 1, 1, sha256Hex),
		gojq.WithFunction("uuid", 0, 0, uuidFunc),
	)
	if err != nil {
		return nil, err
	}
	return &Program{code: code}, nil
}

// Run evaluates the program once and returns its single result. A program
// returning several results contributes only the first, the way a gateway
// rewrite expects one document out.
func (p *Program) Run(input any) (any, error) {
	value, ok := p.code.Run(input).Next()
	if !ok {
		return nil, fmt.Errorf("produced no output")
	}
	if err, ok := value.(error); ok {
		return nil, err
	}
	return value, nil
}

// sha256Hex is the jq function sha256hex(s): the lowercase hex SHA-256 of s's
// UTF-8 bytes. It exists because a program that signs a request (an
// impersonated client's billing block, a body checksum) needs a hash jq does
// not provide.
func sha256Hex(_ any, args []any) any {
	s, ok := args[0].(string)
	if !ok {
		return fmt.Errorf("sha256hex: expected a string, got %T", args[0])
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// uuidFunc is the jq function uuid: a fresh random RFC 4122 version 4 UUID.
// A program that stamps a request with a per-request id, as an impersonated
// client does, needs one.
func uuidFunc(_ any, _ []any) any {
	return NewUUID()
}

// NewUUID returns a random RFC 4122 version 4 UUID, falling back to the nil
// UUID rather than failing when randomness is unavailable.
func NewUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
