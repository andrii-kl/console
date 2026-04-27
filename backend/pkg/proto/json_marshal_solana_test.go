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
	"context"
	"encoding/json"
	"testing"

	"github.com/bufbuild/protocompile"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestMarshalSolanaJSON_Bytes32And64ToBase58(t *testing.T) {
	const protoSrc = `
syntax = "proto3";
package solana_test;

message Trade {
  bytes pool = 1;
  bytes mint_a = 2;
  bytes signature = 3;
  bytes payload = 4;
  repeated bytes pubkeys = 5;
  Inner inner = 6;
}

message Inner {
  bytes inner_pool = 1;
}
`
	md := compileMessage(t, "solana_test.Trade", protoSrc)

	pubkey32 := bytes32(0xAB)
	mintA := bytes32(0xCD)
	signature := bytes64(0xEF)
	payloadShort := []byte{1, 2, 3, 4, 5}
	innerPubkey := bytes32(0x11)
	listPubkey := bytes32(0x22)

	msg := dynamicpb.NewMessage(md)
	setBytes(msg, "pool", pubkey32)
	setBytes(msg, "mint_a", mintA)
	setBytes(msg, "signature", signature)
	setBytes(msg, "payload", payloadShort)

	pubkeysFD := md.Fields().ByName("pubkeys")
	list := msg.Mutable(pubkeysFD).List()
	list.Append(protoreflect.ValueOfBytes(listPubkey))

	innerFD := md.Fields().ByName("inner")
	innerMsg := dynamicpb.NewMessage(innerFD.Message())
	setBytes(innerMsg, "inner_pool", innerPubkey)
	msg.Set(innerFD, protoreflect.ValueOfMessage(innerMsg))

	jsonBytes, err := MarshalSolanaJSON(protojson.MarshalOptions{
		EmitDefaultValues: true,
	}, msg)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &got))

	assert.Equal(t, base58.Encode(pubkey32), got["pool"], "32-byte pubkey -> base58")
	assert.Equal(t, base58.Encode(mintA), got["mintA"], "32-byte mint_a -> base58 (camelCase)")
	assert.Equal(t, base58.Encode(signature), got["signature"], "64-byte signature -> base58")

	// 5-byte payload remains base64 (protojson default)
	payloadVal, ok := got["payload"].(string)
	require.True(t, ok)
	assert.Equal(t, "AQIDBAU=", payloadVal, "non-32/64 byte field stays base64")

	pubkeysVal, ok := got["pubkeys"].([]any)
	require.True(t, ok)
	require.Len(t, pubkeysVal, 1)
	assert.Equal(t, base58.Encode(listPubkey), pubkeysVal[0], "repeated 32-byte -> base58")

	inner, ok := got["inner"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, base58.Encode(innerPubkey), inner["innerPool"], "nested 32-byte -> base58")
}

func TestMarshalSolanaJSON_NoBytesFields(t *testing.T) {
	const protoSrc = `
syntax = "proto3";
package solana_test;
message Plain {
  string name = 1;
  int64 value = 2;
}
`
	md := compileMessage(t, "solana_test.Plain", protoSrc)
	msg := dynamicpb.NewMessage(md)
	msg.Set(md.Fields().ByName("name"), protoreflect.ValueOfString("hello"))
	msg.Set(md.Fields().ByName("value"), protoreflect.ValueOfInt64(42))

	jsonBytes, err := MarshalSolanaJSON(protojson.MarshalOptions{
		EmitDefaultValues: true,
	}, msg)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(jsonBytes, &got))

	assert.Equal(t, "hello", got["name"])
	assert.Equal(t, "42", got["value"]) // int64 serialized as string by protojson
}

func compileMessage(t *testing.T, fullName, source string) protoreflect.MessageDescriptor {
	t.Helper()

	const filename = "test.proto"
	compiler := protocompile.Compiler{
		Resolver: &protocompile.SourceResolver{
			Accessor: protocompile.SourceAccessorFromMap(map[string]string{
				filename: source,
			}),
		},
	}

	files, err := compiler.Compile(context.Background(), filename)
	require.NoError(t, err)
	require.Len(t, files, 1)

	msg := files[0].Messages().ByName(protoreflect.Name(fullName[len("solana_test."):]))
	require.NotNil(t, msg)
	return msg
}

func setBytes(msg *dynamicpb.Message, fieldName string, value []byte) {
	fd := msg.Descriptor().Fields().ByName(protoreflect.Name(fieldName))
	if fd == nil {
		panic("field not found: " + fieldName)
	}
	msg.Set(fd, protoreflect.ValueOfBytes(value))
}

func bytes32(seed byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = seed + byte(i)
	}
	return out
}

func bytes64(seed byte) []byte {
	out := make([]byte, 64)
	for i := range out {
		out[i] = seed + byte(i)
	}
	return out
}

// silence unused import warnings if a future refactor drops a dep.
var _ = proto.Marshal
