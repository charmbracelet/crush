// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package protocol

import (
	"encoding/json"
	"fmt"
)

// DocumentChange is a union of various file edit operations.
//
// Exactly one field of this struct is non-nil; see [DocumentChange.Valid].
//
// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#resourceChanges
type DocumentChange struct {
	TextDocumentEdit *TextDocumentEdit
	CreateFile       *CreateFile
	RenameFile       *RenameFile
	DeleteFile       *DeleteFile
}

// Valid reports whether the DocumentChange sum-type value is valid,
// that is, exactly one of create, delete, edit, or rename.
func (d DocumentChange) Valid() bool {
	n := 0
	if d.TextDocumentEdit != nil {
		n++
	}
	if d.CreateFile != nil {
		n++
	}
	if d.RenameFile != nil {
		n++
	}
	if d.DeleteFile != nil {
		n++
	}
	return n == 1
}

func (d *DocumentChange) UnmarshalJSON(data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	if _, ok := m["textDocument"]; ok {
		d.TextDocumentEdit = new(TextDocumentEdit)
		return json.Unmarshal(data, d.TextDocumentEdit)
	}

	// The {Create,Rename,Delete}File types all share a 'kind' field.
	kind := m["kind"]
	switch kind {
	case "create":
		d.CreateFile = new(CreateFile)
		return json.Unmarshal(data, d.CreateFile)
	case "rename":
		d.RenameFile = new(RenameFile)
		return json.Unmarshal(data, d.RenameFile)
	case "delete":
		d.DeleteFile = new(DeleteFile)
		return json.Unmarshal(data, d.DeleteFile)
	}
	return fmt.Errorf("DocumentChanges: unexpected kind: %q", kind)
}

func (d *DocumentChange) MarshalJSON() ([]byte, error) {
	if d.TextDocumentEdit != nil {
		return json.Marshal(d.TextDocumentEdit)
	} else if d.CreateFile != nil {
		return json.Marshal(d.CreateFile)
	} else if d.RenameFile != nil {
		return json.Marshal(d.RenameFile)
	} else if d.DeleteFile != nil {
		return json.Marshal(d.DeleteFile)
	}
	return nil, fmt.Errorf("empty DocumentChanges union value")
}
