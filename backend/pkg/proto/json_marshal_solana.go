// Copyright 2026 Redpanda Data, Inc.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.md
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0

package proto

import (
	"bytes"
	"encoding/base64"

	"github.com/mr-tron/base58"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// MarshalSolanaJSON marshals a proto message to JSON like protojson, but
// re-encodes bytes fields whose value length is 32 (Solana pubkey) or 64
// (Ed25519 signature) as base58 strings instead of base64. Other bytes lengths
// remain base64. Non-bytes fields and field ordering are preserved exactly as
// emitted by protojson.
func MarshalSolanaJSON(opts protojson.MarshalOptions, msg proto.Message) ([]byte, error) {
	jsonBytes, err := opts.Marshal(msg)
	if err != nil {
		return nil, err
	}

	replacements := make(map[string]string)
	collectSolanaBytesReplacements(msg.ProtoReflect(), replacements)
	if len(replacements) == 0 {
		return jsonBytes, nil
	}

	for from, to := range replacements {
		jsonBytes = bytes.ReplaceAll(jsonBytes, []byte(from), []byte(to))
	}
	return jsonBytes, nil
}

// collectSolanaBytesReplacements walks the populated fields of a message and
// builds a set of base64-quoted → base58-quoted string replacements for each
// bytes field whose value length is 32 or 64.
func collectSolanaBytesReplacements(m protoreflect.Message, out map[string]string) {
	if isWellKnownLeafType(m.Descriptor()) {
		return
	}

	m.Range(func(fd protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			mapVal := val.Map()
			valueFD := fd.MapValue()
			mapVal.Range(func(_ protoreflect.MapKey, v protoreflect.Value) bool {
				collectFromLeaf(valueFD, v, out)
				return true
			})
		case fd.IsList():
			list := val.List()
			for i := 0; i < list.Len(); i++ {
				collectFromLeaf(fd, list.Get(i), out)
			}
		default:
			collectFromLeaf(fd, val, out)
		}
		return true
	})
}

func collectFromLeaf(fd protoreflect.FieldDescriptor, val protoreflect.Value, out map[string]string) {
	switch fd.Kind() {
	case protoreflect.BytesKind:
		b := val.Bytes()
		if len(b) != 32 && len(b) != 64 {
			return
		}
		// Quote the strings so we never replace partial matches inside other tokens.
		from := `"` + base64.StdEncoding.EncodeToString(b) + `"`
		to := `"` + base58.Encode(b) + `"`
		out[from] = to
	case protoreflect.MessageKind, protoreflect.GroupKind:
		collectSolanaBytesReplacements(val.Message(), out)
	}
}

// isWellKnownLeafType returns true for protobuf well-known types whose JSON
// representation is not a regular field-by-field object. We still let the
// replacement scan touch them — it's a string-level pass that only matches
// quoted base64 of length 44 or 88 chars, so descending into them is harmless,
// but skipping the recursion avoids confusion if their internal Go shapes
// change in future SDK versions.
func isWellKnownLeafType(md protoreflect.MessageDescriptor) bool {
	switch md.FullName() {
	case "google.protobuf.Any",
		"google.protobuf.Timestamp",
		"google.protobuf.Duration",
		"google.protobuf.FieldMask",
		"google.protobuf.Struct",
		"google.protobuf.Value",
		"google.protobuf.ListValue",
		"google.protobuf.NullValue",
		"google.protobuf.BoolValue",
		"google.protobuf.StringValue",
		"google.protobuf.BytesValue",
		"google.protobuf.Int32Value",
		"google.protobuf.Int64Value",
		"google.protobuf.UInt32Value",
		"google.protobuf.UInt64Value",
		"google.protobuf.FloatValue",
		"google.protobuf.DoubleValue",
		"google.protobuf.Empty":
		return true
	}
	return false
}
