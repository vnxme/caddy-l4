// Copyright 2024 VNXME
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package l4quic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"go.uber.org/zap"

	"github.com/mholt/caddy-l4/layer4"
)

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("Unexpected error: %s\n", err)
	}
}

func Test_MatchQUIC_Match(t *testing.T) {
	type test struct {
		matcher     *MatchQUIC
		data        []byte
		shouldMatch bool
	}

	m0 := &MatchQUIC{}
	m1 := &MatchQUIC{MatchersRaw: map[string]json.RawMessage{
		"sni": json.RawMessage(`["example.com"]`),
	}}
	m2 := &MatchQUIC{MatchersRaw: map[string]json.RawMessage{
		"alpn": json.RawMessage(`["custom"]`),
		"sni":  json.RawMessage(`["example.com"]`),
	}}
	m3 := &MatchQUIC{MatchersRaw: map[string]json.RawMessage{
		"alpn": json.RawMessage(`["h3"]`),
		"sni":  json.RawMessage(`["example.com"]`),
	}}

	tests := []test{
		{matcher: m0, data: packet1, shouldMatch: true},
		{matcher: m0, data: packet2, shouldMatch: true},
		{matcher: m0, data: packet3, shouldMatch: true},

		{matcher: m1, data: packet1, shouldMatch: true},
		{matcher: m1, data: packet2, shouldMatch: true},
		{matcher: m1, data: packet3, shouldMatch: true},

		{matcher: m2, data: packet1, shouldMatch: false},
		{matcher: m2, data: packet2, shouldMatch: true},
		{matcher: m2, data: packet3, shouldMatch: false},

		{matcher: m3, data: packet1, shouldMatch: true},
		{matcher: m3, data: packet2, shouldMatch: false},
		{matcher: m3, data: packet3, shouldMatch: true},

		{matcher: m0, data: append(packet1, packet2...), shouldMatch: false},
		{matcher: m0, data: append([]byte{QUICMagicBitValue}, packet1[1:]...), shouldMatch: false},
		{matcher: m0, data: append([]byte{QUICLongHeaderBitValue}, packet1[1:]...), shouldMatch: false},
		{matcher: m0, data: append([]byte{QUICMagicBitValue | QUICLongHeaderBitValue}, packet1[1:]...), shouldMatch: false},
		{matcher: m0, data: packet1[:len(packet1)-1], shouldMatch: false},
	}
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	for i, tc := range tests {
		func() {
			err := tc.matcher.Provision(ctx)
			assertNoError(t, err)

			in, out := newFakePacketConnPipe(&net.UDPAddr{}, nil)
			defer func() {
				_, _ = io.Copy(io.Discard, out)
				_ = out.Close()
			}()

			cx := layer4.WrapConnection(out, []byte{}, zap.NewNop())
			go func() {
				_, err := in.Write(tc.data)
				assertNoError(t, err)
				_ = in.Close()
			}()

			matched, err := tc.matcher.Match(cx)
			assertNoError(t, err)

			if matched != tc.shouldMatch {
				if tc.shouldMatch {
					t.Fatalf("Test %d: matcher did not match | %+v\n", i, tc.matcher)
				} else {
					t.Fatalf("Test %d: matcher should not match | %+v\n", i, tc.matcher)
				}
			}
		}()
	}
}

// quicreach example.com --stats --unsecure --ip 127.0.0.1 --port 8443 [--alpn h3]
var packet1 = []byte{0xc4, 0x00, 0x00, 0x00, 0x01, 0x08, 0x1b, 0xb6, 0x86, 0x97, 0xca, 0xc4, 0xa6, 0xf8, 0x00, 0x00, 0x44, 0xda, 0xf2, 0x95, 0xc1, 0xe3, 0xe5, 0x13, 0x99, 0x6d, 0x06, 0x6b, 0x15, 0x80, 0x78, 0xbc, 0x17, 0x22, 0x5c, 0xad, 0xb3, 0x34, 0xc9, 0x08, 0x2b, 0x76, 0x6f, 0x91, 0x29, 0xea, 0xac, 0x52, 0xc8, 0x7b, 0x23, 0xe6, 0x76, 0xa6, 0x64, 0xc8, 0xb0, 0x30, 0x2d, 0x52, 0x7f, 0x18, 0x0f, 0x1d, 0x5b, 0xb4, 0x61, 0x05, 0x32, 0x4e, 0xc0, 0xe2, 0x4f, 0x46, 0x21, 0x22, 0x73, 0x75, 0xcd, 0xd3, 0xdf, 0xbd, 0x16, 0x6f, 0x9f, 0xdf, 0xa9, 0x32, 0xe9, 0x15, 0xd1, 0xc9, 0x70, 0x86, 0x0d, 0x1e, 0xb5, 0xf0, 0x01, 0x5b, 0x28, 0xe3, 0x21, 0x97, 0xaa, 0xa1, 0x3e, 0x8b, 0x96, 0xf3, 0x6a, 0x7d, 0x3a, 0x7f, 0xa6, 0xd4, 0xf6, 0x34, 0x1a, 0x17, 0x91, 0x59, 0x74, 0x7c, 0x9b, 0x99, 0x05, 0x36, 0xf4, 0xc5, 0x23, 0xd1, 0x84, 0x14, 0x58, 0x82, 0xff, 0x79, 0x7d, 0x95, 0xcd, 0x9b, 0x66, 0xdb, 0xcf, 0x0b, 0xf6, 0xf2, 0xfd, 0x20, 0x33, 0x7f, 0x5a, 0xab, 0x4c, 0x02, 0x37, 0xd3, 0x8e, 0xcb, 0x17, 0x7c, 0x03, 0xd2, 0xfc, 0x89, 0xaa, 0x73, 0x6b, 0xef, 0x76, 0xfc, 0xed, 0x8e, 0xd8, 0x0b, 0xf5, 0xe5, 0xb8, 0x9c, 0xdc, 0x92, 0xc5, 0xb3, 0x61, 0xba, 0x8d, 0xc3, 0xab, 0xae, 0x54, 0x57, 0xa1, 0x0e, 0x61, 0xf9, 0xd7, 0xc5, 0xd1, 0x5e, 0xcf, 0xcd, 0x2d, 0x68, 0x52, 0x33, 0x7e, 0xc9, 0x32, 0x7f, 0x70, 0x60, 0xea, 0xed, 0x9f, 0x6b, 0x3a, 0x6b, 0xa3, 0x24, 0x58, 0xb3, 0x58, 0x88, 0x49, 0xa3, 0x75, 0xcb, 0x94, 0x19, 0x47, 0x6e, 0xd1, 0x6f, 0x13, 0x3f, 0xf3, 0x20, 0xa4, 0x96, 0x6f, 0xce, 0xac, 0x35, 0x3e, 0x86, 0x08, 0xcd, 0x5f, 0xe0, 0xbf, 0x99, 0x91, 0x6b, 0x3a, 0xb6, 0x41, 0x74, 0x40, 0x5a, 0xfd, 0x53, 0x37, 0xff, 0xac, 0x96, 0x59, 0xc3, 0xcf, 0xb4, 0x3b, 0xf4, 0xbe, 0x31, 0x7a, 0x41, 0xf8, 0x2b, 0xc9, 0x5e, 0x20, 0x65, 0x34, 0xdb, 0xbd, 0xdc, 0xec, 0x49, 0x95, 0x0d, 0x03, 0xbe, 0x7e, 0xbd, 0xfe, 0x0d, 0x16, 0x46, 0x53, 0x60, 0x49, 0xe6, 0xfc, 0xdc, 0x81, 0xe5, 0x3f, 0x26, 0x03, 0x9f, 0xcb, 0xd1, 0xe7, 0xf0, 0x75, 0xe9, 0x4e, 0x42, 0x59, 0x65, 0x84, 0xac, 0x3b, 0x0b, 0xdb, 0xc2, 0x70, 0xb0, 0x39, 0xfa, 0x2a, 0xa4, 0x4e, 0xee, 0xfe, 0x34, 0x1d, 0x55, 0x71, 0x1b, 0xc4, 0xb1, 0x61, 0xb7, 0x7a, 0xb1, 0x98, 0x46, 0xc0, 0x44, 0x8f, 0xb5, 0xef, 0xf3, 0xf8, 0x71, 0x73, 0xef, 0x9f, 0x40, 0x86, 0xf6, 0xe0, 0x9d, 0xc4, 0x2e, 0xc1, 0x87, 0x7a, 0xb8, 0xfb, 0x19, 0x89, 0xca, 0xc8, 0xcf, 0x98, 0x05, 0x48, 0x6f, 0x53, 0x3a, 0xed, 0x09, 0x79, 0xbd, 0x32, 0xf5, 0x7b, 0xda, 0x8a, 0xe9, 0x5b, 0x40, 0x14, 0xd5, 0xb8, 0x0b, 0xc5, 0xb6, 0xb3, 0xd9, 0x7c, 0xc9, 0x4e, 0x0a, 0xc1, 0x9f, 0x3d, 0x9a, 0x8f, 0x50, 0xf5, 0x15, 0x05, 0x59, 0x78, 0xd4, 0x2e, 0x25, 0xe1, 0x07, 0xf1, 0x9d, 0x52, 0x15, 0x85, 0xcd, 0x81, 0x0f, 0xe6, 0x39, 0x14, 0x31, 0xb2, 0xf0, 0xaf, 0x84, 0x94, 0xf0, 0x6d, 0x6a, 0x70, 0xbf, 0xdc, 0xcc, 0x21, 0xdd, 0xc8, 0xdb, 0x0b, 0x8e, 0xb7, 0x0f, 0x11, 0x0f, 0x61, 0x09, 0xef, 0xb2, 0x27, 0x2c, 0x68, 0x0f, 0x33, 0x99, 0x82, 0x96, 0xd6, 0x94, 0xf9, 0x3f, 0x14, 0xc1, 0xaf, 0xe4, 0x3a, 0x41, 0xc4, 0xd9, 0x76, 0xcc, 0xb1, 0x8f, 0x81, 0x3f, 0x56, 0x41, 0xc6, 0x62, 0x1c, 0xfc, 0x1a, 0x31, 0x12, 0x10, 0x59, 0x14, 0xc1, 0xb4, 0x3b, 0x0f, 0x40, 0xb8, 0xca, 0xe0, 0x27, 0x0d, 0x06, 0x91, 0x4c, 0x2f, 0xc7, 0x3f, 0x0b, 0xc8, 0x4c, 0x94, 0x3d, 0xc9, 0x10, 0x12, 0x31, 0xde, 0x89, 0xa4, 0xdb, 0x47, 0xe4, 0xfe, 0x46, 0x8f, 0x7c, 0xa9, 0x32, 0xb4, 0x99, 0x3b, 0x2a, 0xef, 0xe1, 0x2b, 0x09, 0xba, 0x00, 0xbb, 0x45, 0x72, 0xd2, 0xeb, 0xff, 0xdf, 0xae, 0xec, 0x3e, 0xdc, 0xbf, 0x18, 0xce, 0xe8, 0x53, 0x15, 0xd6, 0x26, 0xdb, 0x93, 0x29, 0xd0, 0xe1, 0x4d, 0xe3, 0x57, 0x2f, 0xb1, 0x9b, 0xb8, 0x65, 0x8e, 0x2f, 0x7c, 0x9c, 0x94, 0x42, 0x91, 0x04, 0xc1, 0xce, 0x8e, 0xa2, 0x18, 0x88, 0xa1, 0x2a, 0xc2, 0xe3, 0xb4, 0x38, 0xe1, 0xb6, 0xd6, 0x37, 0x83, 0xc0, 0x59, 0x7f, 0x99, 0x6d, 0x1b, 0xf5, 0x8e, 0x4f, 0x71, 0x74, 0x58, 0x8f, 0xc3, 0x2b, 0x94, 0x23, 0x34, 0x8d, 0xa8, 0xa4, 0xae, 0x32, 0x48, 0x9f, 0x81, 0xf6, 0xde, 0xa6, 0x01, 0x1c, 0x3f, 0x5c, 0xa7, 0x8a, 0xbb, 0xde, 0xc0, 0xaf, 0x1a, 0x95, 0x3e, 0x86, 0x6b, 0x09, 0x27, 0xa2, 0x01, 0xaf, 0x38, 0xf5, 0x28, 0xaf, 0xd1, 0xe1, 0xd8, 0xd1, 0xbc, 0x77, 0xca, 0xc5, 0x17, 0xf1, 0x12, 0xf1, 0x4d, 0x86, 0x46, 0x86, 0xb0, 0xc1, 0xbb, 0xf8, 0x5a, 0x7c, 0x0e, 0x36, 0x23, 0x90, 0xa8, 0x69, 0xc2, 0x0c, 0x98, 0x04, 0xc7, 0x8c, 0x92, 0x14, 0xeb, 0x4c, 0x1a, 0x53, 0x02, 0x0c, 0x9d, 0xf1, 0x16, 0x1b, 0xc6, 0xb4, 0x85, 0x09, 0xf1, 0xb7, 0x99, 0x83, 0x5d, 0x64, 0x27, 0x42, 0x61, 0x07, 0x29, 0xb1, 0x2f, 0x23, 0xc3, 0xb5, 0xe7, 0xf5, 0xf8, 0x8b, 0x6f, 0x6c, 0xc4, 0x81, 0x7b, 0x48, 0x1a, 0x90, 0x3e, 0xb0, 0xbc, 0xb2, 0x9b, 0xf8, 0xf8, 0x28, 0x40, 0x78, 0x3a, 0x15, 0x89, 0x1a, 0x79, 0x06, 0x39, 0xfe, 0x67, 0x13, 0x02, 0x0e, 0x6f, 0x24, 0xf0, 0x0a, 0x36, 0x40, 0x00, 0xb2, 0x49, 0x0c, 0xf2, 0xd7, 0x1f, 0x58, 0x03, 0x5e, 0x04, 0xd1, 0xf2, 0x1f, 0x7e, 0x93, 0x08, 0x1d, 0xf6, 0xf7, 0x15, 0xed, 0xf2, 0xf3, 0x32, 0x37, 0x64, 0xe4, 0x27, 0x0a, 0xcb, 0xf4, 0x2b, 0xe1, 0xfc, 0xb3, 0x31, 0x0c, 0x02, 0x2e, 0x6f, 0x2a, 0xc5, 0x55, 0x09, 0x8b, 0xb7, 0x80, 0xc7, 0x86, 0xf4, 0x8b, 0x8a, 0xc5, 0x20, 0xb3, 0xf9, 0x47, 0xf6, 0x83, 0x7b, 0x22, 0xfc, 0xef, 0x20, 0x80, 0x6b, 0x13, 0xbc, 0x8c, 0xd9, 0xc7, 0x9b, 0x81, 0x13, 0x95, 0x12, 0xf6, 0xbc, 0x26, 0xba, 0xdc, 0x10, 0xcd, 0xf7, 0xca, 0xd1, 0x41, 0xe0, 0xfc, 0x20, 0x7d, 0x87, 0xf5, 0xf0, 0x78, 0x8e, 0x0f, 0xa1, 0x3d, 0x91, 0x3b, 0x84, 0xfd, 0xb4, 0xb5, 0x20, 0xa0, 0x3e, 0x61, 0x2d, 0x62, 0x32, 0xae, 0x64, 0xa5, 0xcf, 0xa6, 0x9c, 0xe2, 0x54, 0x36, 0x32, 0x29, 0x5b, 0x58, 0x19, 0xb3, 0xc2, 0x97, 0x63, 0xb9, 0x49, 0xd6, 0x02, 0x02, 0xe2, 0x34, 0xd1, 0xf6, 0x14, 0xc5, 0x93, 0x91, 0xb4, 0xac, 0x38, 0xc6, 0x97, 0x26, 0x91, 0xb7, 0xef, 0xbb, 0xf4, 0xae, 0x66, 0xc2, 0xdb, 0xa4, 0xd5, 0x3c, 0x8f, 0x60, 0x5a, 0x0b, 0xb2, 0x1f, 0xcd, 0x3f, 0x34, 0x67, 0x65, 0x97, 0x9c, 0x48, 0xf7, 0x07, 0x52, 0x5e, 0xae, 0x31, 0xe5, 0x56, 0x3a, 0xa2, 0x92, 0x60, 0xa2, 0x49, 0xef, 0x43, 0xc1, 0xaa, 0xc0, 0xe6, 0x78, 0x5c, 0x39, 0x4b, 0xa6, 0x5e, 0xae, 0x9f, 0x58, 0x59, 0x6f, 0x55, 0x7e, 0x6a, 0x02, 0xd6, 0x8b, 0x9b, 0xc9, 0x09, 0x82, 0xca, 0xad, 0x9f, 0x41, 0x49, 0x4b, 0x2d, 0x33, 0x08, 0x6d, 0xd9, 0xd8, 0x2c, 0xb3, 0xe4, 0xf1, 0xe8, 0x74, 0x3b, 0x42, 0x0b, 0x0e, 0x58, 0xe2, 0x70, 0x9c, 0xf9, 0xa2, 0x57, 0x53, 0x7d, 0xcd, 0xb7, 0x7d, 0x89, 0x6e, 0x96, 0xb8, 0xb6, 0xdc, 0x82, 0xb9, 0x0e, 0x6e, 0x80, 0x53, 0xa2, 0xf8, 0x99, 0x8b, 0xcc, 0x4a, 0x57, 0xa9, 0xde, 0x22, 0x35, 0x53, 0xb0, 0xda, 0x55, 0x94, 0x60, 0xb0, 0x26, 0x20, 0xb7, 0xae, 0x50, 0x65, 0xca, 0x79, 0x83, 0xf2, 0x6e, 0x62, 0x11, 0x54, 0x6e, 0xca, 0xaf, 0xe2, 0xf7, 0x1c, 0xe0, 0x9e, 0xf7, 0xd3, 0x9d, 0x26, 0x3b, 0x4c, 0xd0, 0xe2, 0xa7, 0x8f, 0x29, 0x91, 0x4a, 0x7b, 0xda, 0x58, 0x9f, 0x64, 0x11, 0xaf, 0xcb, 0x75, 0xb5, 0xab, 0x8c, 0x91, 0x44, 0x6d, 0xa7, 0xd7, 0x29, 0x60, 0xf9, 0x82, 0x47, 0x1e, 0x75, 0x8c, 0xc4, 0x7e, 0x99, 0x8f, 0x6a, 0x52, 0xd6, 0xf1, 0x2b, 0x7c, 0xd2, 0xec, 0x74, 0xf0, 0x49, 0x9e, 0x52, 0x8d, 0x89, 0x1a, 0x9b, 0x1e, 0xf7, 0x9f, 0xfe, 0x4c, 0xa9, 0xe2, 0xa8, 0xca, 0x53, 0x54, 0x42, 0xd7, 0x50, 0x8b, 0xe3, 0x32, 0x47, 0xd8, 0x5e, 0x89, 0xb2, 0x6a, 0xff, 0x97, 0xa5, 0xaf, 0x5b, 0x0d, 0x59, 0x71, 0xb0, 0x67, 0xdb, 0xcb, 0xf4, 0xc6, 0x90, 0xc1, 0xbd, 0x67, 0xf2, 0x44, 0x4e, 0x24, 0xe4, 0x3e, 0x68, 0xb4, 0x4b, 0x7f, 0x7f, 0x21, 0x55, 0xe5, 0xed, 0xc3, 0x5f, 0x92, 0xa6, 0x23, 0x67, 0x5f, 0x76, 0x29, 0x83, 0xc9, 0xd7, 0x76, 0x0e, 0xec, 0xad, 0x2e, 0x43, 0xe7, 0xe4, 0x0a, 0xb6, 0x77, 0xff, 0x82, 0x11, 0x01, 0x92, 0x4f, 0x23, 0x9f, 0x1a, 0x04, 0x9b, 0x02, 0x17, 0xa6, 0xdd, 0x2b, 0xd9, 0xc4, 0x19, 0xd8, 0xf8, 0x17, 0x61, 0x82, 0xc9, 0x3f, 0x02, 0xe1, 0xc4, 0x12, 0x53, 0xaf, 0xf3, 0x09, 0x48, 0x19, 0x29, 0x9e, 0x85, 0x70, 0x8a, 0x6a, 0xbd, 0xad, 0x85, 0xcc, 0x5e, 0x19, 0xaa, 0xb5, 0x0a, 0x00, 0x9e, 0x16, 0xb1, 0x36, 0x07, 0xf4}

// quicreach example.com --stats --unsecure --ip 127.0.0.1 --port 8443 --alpn custom
var packet2 = []byte{0xc5, 0x00, 0x00, 0x00, 0x01, 0x08, 0x0b, 0x97, 0xd9, 0xb1, 0x92, 0x01, 0x79, 0x9c, 0x00, 0x00, 0x44, 0xda, 0xe4, 0x4b, 0x65, 0x9f, 0x2a, 0xb3, 0xb0, 0x25, 0xeb, 0xc6, 0xe6, 0xfe, 0x8d, 0xab, 0x3f, 0x21, 0x2e, 0xca, 0xb4, 0x2b, 0xae, 0xa4, 0xbf, 0x18, 0x0c, 0x0e, 0xbf, 0x61, 0xd6, 0x96, 0xd2, 0x69, 0xfc, 0x71, 0x69, 0x0b, 0x3d, 0x54, 0x4e, 0x8d, 0x20, 0x87, 0x02, 0x4a, 0x6b, 0xb4, 0xcb, 0x00, 0x97, 0xb3, 0x1c, 0x15, 0xc6, 0x12, 0x08, 0xfa, 0x77, 0xe5, 0x83, 0xa9, 0xa6, 0x74, 0x8d, 0xb7, 0x76, 0x20, 0x11, 0x09, 0xd3, 0x07, 0x30, 0xba, 0xe2, 0xb0, 0x8b, 0xc8, 0x4f, 0x18, 0x3a, 0xfe, 0xed, 0xcc, 0xc4, 0x0e, 0x1a, 0xf0, 0x56, 0xc9, 0xa2, 0x49, 0xc6, 0x60, 0x6e, 0x17, 0x1e, 0x66, 0xfb, 0x5d, 0x5c, 0xdf, 0xbb, 0xd0, 0x39, 0x45, 0x27, 0xcc, 0x21, 0x94, 0xe7, 0x71, 0xd6, 0xbd, 0xb1, 0x6b, 0x39, 0xaf, 0x77, 0x2e, 0x67, 0x06, 0xbc, 0x9c, 0xee, 0x16, 0x97, 0x61, 0x9c, 0x1b, 0x29, 0x9b, 0xef, 0xc9, 0x46, 0xfe, 0xbe, 0xe0, 0xa5, 0x04, 0x4f, 0x13, 0x9e, 0x32, 0x0b, 0x00, 0x2e, 0xf0, 0xb7, 0x0a, 0x04, 0x63, 0xa0, 0xb3, 0xbc, 0xf9, 0x50, 0xef, 0x34, 0x97, 0x32, 0x02, 0x01, 0x51, 0x59, 0x85, 0x55, 0x0a, 0xd5, 0xa2, 0x87, 0x9b, 0xed, 0x09, 0x38, 0x2a, 0x50, 0x64, 0xbf, 0x63, 0xe6, 0xa0, 0xa1, 0x07, 0x66, 0xe8, 0x5b, 0xbc, 0xb6, 0x71, 0xb4, 0x9c, 0xa9, 0x50, 0xca, 0xe5, 0x70, 0x5c, 0x31, 0x7d, 0xeb, 0xf6, 0x16, 0x3b, 0xad, 0x9d, 0x5d, 0x9f, 0x90, 0x4c, 0xc0, 0x87, 0x22, 0x00, 0x29, 0x72, 0x03, 0x6e, 0x26, 0x5c, 0x4e, 0x63, 0xc8, 0xe4, 0x8c, 0x9e, 0x1e, 0xb0, 0x40, 0x38, 0xd1, 0xaa, 0x8b, 0xf5, 0xde, 0x6a, 0x4e, 0x3e, 0x85, 0x1b, 0x70, 0x7f, 0xa1, 0x8a, 0xab, 0xba, 0x5c, 0xd2, 0x3b, 0x67, 0xd0, 0x99, 0x69, 0xf6, 0x39, 0x96, 0x1b, 0xf7, 0x1c, 0xf6, 0xda, 0x3b, 0x4e, 0x44, 0x71, 0xd0, 0x03, 0x0c, 0xa0, 0x26, 0x5d, 0xc7, 0xb5, 0x80, 0x6f, 0xce, 0x90, 0x52, 0xb8, 0x6e, 0xb1, 0xb4, 0x80, 0x70, 0x21, 0xd1, 0x6d, 0xd6, 0x1c, 0xa0, 0x55, 0x7d, 0xac, 0xf6, 0xa7, 0x88, 0x2e, 0x88, 0x6d, 0x25, 0x8d, 0x6d, 0x96, 0x4e, 0x00, 0xd6, 0x1b, 0x4c, 0x78, 0xc5, 0xec, 0x18, 0x5f, 0xc0, 0xb7, 0x48, 0xad, 0xbf, 0x3c, 0xee, 0x4e, 0xca, 0x2d, 0x0f, 0x39, 0x00, 0xdd, 0x84, 0x3f, 0x24, 0xf2, 0x86, 0xad, 0x42, 0x69, 0x00, 0x64, 0x27, 0x72, 0x2d, 0xb5, 0xfd, 0x3e, 0xd0, 0xb2, 0x31, 0xa1, 0xa8, 0xf3, 0x26, 0x9e, 0x57, 0xc0, 0x46, 0x79, 0x52, 0x79, 0x50, 0xec, 0x7d, 0x4a, 0x22, 0x6b, 0xea, 0x9f, 0x48, 0xa1, 0x71, 0x95, 0x88, 0x1c, 0xb2, 0xc6, 0x62, 0x49, 0x6f, 0xc6, 0xbe, 0xae, 0xd0, 0x10, 0x89, 0xff, 0xb0, 0x1e, 0x56, 0x26, 0x03, 0x40, 0x5d, 0xfc, 0x55, 0xc6, 0xe1, 0x86, 0x25, 0x8a, 0xcb, 0x49, 0xa7, 0x52, 0xe3, 0xda, 0x6a, 0x35, 0x8e, 0x70, 0x43, 0xef, 0x32, 0x46, 0x61, 0xc3, 0xab, 0x63, 0x5b, 0x91, 0xe0, 0x7d, 0xe2, 0x48, 0x3d, 0xf6, 0xf8, 0x84, 0x78, 0xf6, 0xb2, 0x53, 0x3e, 0xda, 0xa5, 0xdf, 0xb0, 0x65, 0x7b, 0xe7, 0x3b, 0x8f, 0x95, 0x51, 0x3a, 0x86, 0xc5, 0xc1, 0x9c, 0xe2, 0x3c, 0x4f, 0x35, 0xe5, 0x02, 0xeb, 0xd8, 0xeb, 0xaf, 0xe6, 0x1e, 0x2f, 0xb8, 0x58, 0x0b, 0x2c, 0x08, 0xe8, 0xda, 0xea, 0x1b, 0x31, 0x57, 0x37, 0xb4, 0x00, 0xdc, 0xbd, 0xfd, 0x11, 0xca, 0xaf, 0x05, 0x18, 0x61, 0x9d, 0x00, 0x9a, 0x6d, 0x16, 0x9b, 0xb0, 0xec, 0x66, 0x3a, 0xba, 0xc0, 0x75, 0x53, 0x84, 0xee, 0xfd, 0x46, 0xbe, 0xc9, 0x43, 0x4d, 0x79, 0x46, 0x09, 0xd1, 0xcf, 0xda, 0x62, 0x53, 0xe8, 0x88, 0xb2, 0x8a, 0x83, 0x3a, 0x7f, 0xb8, 0x9d, 0xdc, 0x02, 0x6c, 0x95, 0xe3, 0xea, 0x82, 0xfd, 0x5a, 0xbf, 0xab, 0xe9, 0xe6, 0x01, 0xe5, 0x27, 0x24, 0x18, 0xb1, 0x35, 0x3b, 0x31, 0x23, 0xd8, 0x8c, 0xb0, 0xfb, 0x0c, 0x19, 0x92, 0xcc, 0x18, 0x8d, 0xb7, 0x7d, 0xa9, 0x58, 0x92, 0xbf, 0x67, 0x87, 0xb1, 0x72, 0xbc, 0x70, 0xac, 0x86, 0xa9, 0xa1, 0x7d, 0x59, 0xcf, 0xa7, 0x1d, 0x13, 0xb5, 0x10, 0x98, 0xef, 0xaa, 0xd7, 0x8d, 0x04, 0x62, 0xfa, 0x2a, 0x55, 0xf0, 0xaf, 0xaf, 0xe0, 0xab, 0x87, 0x9d, 0x97, 0xb4, 0xea, 0xbe, 0xd8, 0x35, 0x97, 0x1f, 0xee, 0x7f, 0x11, 0x3f, 0xcc, 0x66, 0x6e, 0xd5, 0xc9, 0xc5, 0xdb, 0x05, 0xf1, 0xef, 0x71, 0x86, 0x54, 0x04, 0x21, 0xc3, 0x7a, 0x56, 0x5a, 0x33, 0x86, 0x2e, 0xca, 0xfb, 0x65, 0x9e, 0xa9, 0x9e, 0xdf, 0x0f, 0x5a, 0x3f, 0x9f, 0x79, 0x8f, 0x23, 0xf8, 0x29, 0xb8, 0xf5, 0x01, 0x7f, 0xed, 0xec, 0xad, 0x3f, 0x01, 0xb9, 0x10, 0x74, 0x72, 0xff, 0xc6, 0x17, 0x72, 0x25, 0x7d, 0x9b, 0x77, 0xde, 0x5a, 0x08, 0xaf, 0x37, 0x7e, 0x3b, 0x9c, 0x5a, 0xbb, 0xdd, 0xdc, 0xd1, 0x8f, 0x46, 0x03, 0x9d, 0x05, 0x8b, 0xe0, 0x93, 0x1a, 0x7d, 0x1d, 0xe4, 0x01, 0xd6, 0x51, 0xf8, 0x41, 0xe1, 0xce, 0xe4, 0xd6, 0xec, 0x53, 0x94, 0x9f, 0x28, 0x0a, 0x64, 0xd9, 0x68, 0xe8, 0x45, 0x16, 0xa1, 0x78, 0xc6, 0x6b, 0x51, 0x37, 0xee, 0x1f, 0x7d, 0x6d, 0x70, 0x59, 0x3f, 0xce, 0xd8, 0xc8, 0xdf, 0x14, 0xbf, 0x54, 0xcb, 0x47, 0x9e, 0xfe, 0x4f, 0xd6, 0x91, 0xc3, 0x84, 0xb9, 0x3a, 0xb1, 0x5a, 0x10, 0x24, 0x1d, 0x76, 0x4f, 0xec, 0x64, 0xa1, 0xea, 0x1b, 0xbc, 0x93, 0x52, 0x8e, 0x54, 0x1e, 0xbc, 0x5c, 0x1f, 0xfa, 0x66, 0x88, 0x35, 0x1b, 0xfe, 0xf5, 0x7c, 0xea, 0x26, 0x96, 0x4d, 0x3c, 0x5a, 0x2b, 0x4d, 0x80, 0x09, 0xde, 0x23, 0xb8, 0x9f, 0x6b, 0x5d, 0xf0, 0xc9, 0xc2, 0x4b, 0x33, 0xbe, 0x89, 0x41, 0x37, 0x7e, 0xdf, 0xce, 0x3e, 0x51, 0x7e, 0x38, 0x1a, 0xf6, 0x98, 0x77, 0xfe, 0x91, 0x58, 0xb1, 0xa7, 0x0d, 0xdb, 0x45, 0xe0, 0x75, 0x95, 0x83, 0x21, 0x87, 0x43, 0x81, 0x55, 0x53, 0xaa, 0x51, 0x92, 0x2c, 0xb2, 0x57, 0xa4, 0x07, 0x87, 0x73, 0xd8, 0xd4, 0x92, 0x71, 0x09, 0x77, 0x7c, 0xe2, 0x89, 0x5b, 0x83, 0x31, 0xf4, 0x38, 0xb4, 0xc8, 0x99, 0xf2, 0x78, 0xdb, 0x19, 0x66, 0x63, 0x15, 0xa5, 0x2b, 0x76, 0x11, 0x07, 0x11, 0x05, 0xa6, 0xd0, 0xe2, 0xae, 0x73, 0xd6, 0x5d, 0x3b, 0xa6, 0x6e, 0x88, 0x46, 0xf9, 0x0a, 0xff, 0xc7, 0xea, 0x56, 0xca, 0x93, 0xd3, 0x9f, 0x2d, 0xb3, 0x7f, 0x2c, 0x99, 0xd9, 0xac, 0x2c, 0x5d, 0x08, 0x06, 0x92, 0x1e, 0x9e, 0x7d, 0x7a, 0x97, 0x6f, 0x29, 0x54, 0x88, 0xdf, 0xf8, 0x04, 0xb1, 0x65, 0x84, 0x2f, 0x5d, 0xa2, 0x2c, 0xd7, 0x6b, 0x55, 0x71, 0x38, 0x5a, 0x37, 0x58, 0x6d, 0xa5, 0x63, 0xb4, 0xe3, 0x4c, 0x8a, 0xff, 0x6f, 0x55, 0xf7, 0x01, 0x9c, 0xaa, 0x7a, 0x76, 0xe7, 0x19, 0x6a, 0xc9, 0x0c, 0x72, 0x07, 0x93, 0xb4, 0x38, 0x52, 0x37, 0x47, 0x6c, 0x00, 0xdd, 0xac, 0x0c, 0x8a, 0x26, 0x0e, 0x16, 0x77, 0x9e, 0x94, 0x63, 0xd9, 0xaf, 0x05, 0x7a, 0x5e, 0xcf, 0xb1, 0x61, 0xd6, 0x26, 0x30, 0xad, 0xd2, 0x73, 0xc0, 0xde, 0x19, 0xe9, 0xc9, 0x89, 0x7e, 0x16, 0xae, 0x2b, 0x79, 0x63, 0x3b, 0x76, 0x8e, 0xd9, 0xa5, 0x93, 0x3a, 0x3d, 0xfa, 0x3a, 0x4d, 0xe8, 0x92, 0xcb, 0xe3, 0xe3, 0x0d, 0x5e, 0xd2, 0x75, 0x61, 0x92, 0xca, 0x65, 0x35, 0xcc, 0xb2, 0xa4, 0x56, 0x77, 0xd1, 0x8e, 0xcb, 0x39, 0x58, 0x7d, 0x1b, 0x48, 0x10, 0xde, 0x7b, 0xff, 0x26, 0xae, 0x02, 0xd3, 0x84, 0x71, 0x1f, 0x3f, 0x13, 0x3a, 0xf4, 0xc8, 0xdf, 0x6a, 0xe3, 0xa8, 0xe2, 0x23, 0xc8, 0xf2, 0x16, 0x45, 0xc5, 0xac, 0xc0, 0xcb, 0xfb, 0x1f, 0x13, 0x4a, 0x90, 0xc8, 0x88, 0xb4, 0x49, 0xca, 0xb7, 0x0e, 0xec, 0x7b, 0x3f, 0x77, 0x0b, 0xc4, 0x9d, 0x07, 0x84, 0x89, 0xac, 0x59, 0xf0, 0x40, 0x34, 0x72, 0x33, 0xc6, 0x7f, 0xdf, 0x39, 0x90, 0x5d, 0x4f, 0x93, 0x4f, 0x81, 0x53, 0x18, 0xa7, 0xc8, 0x8e, 0xd0, 0x08, 0x18, 0x6f, 0x00, 0x6b, 0x77, 0x6a, 0xc7, 0xe7, 0xee, 0xdb, 0xa1, 0x5d, 0xb4, 0xc9, 0xdd, 0x61, 0x22, 0x19, 0x5b, 0xc1, 0xeb, 0xd8, 0xc6, 0x71, 0x0c, 0x8b, 0x48, 0x80, 0xfe, 0x87, 0x75, 0x68, 0x3e, 0x8f, 0x1b, 0xa7, 0xa4, 0xfd, 0x35, 0x15, 0x45, 0xf0, 0x78, 0xe8, 0x34, 0xec, 0x01, 0xab, 0x1d, 0x8e, 0x2a, 0x4b, 0xfe, 0xb8, 0x93, 0x1c, 0xbd, 0x65, 0x38, 0x99, 0x07, 0xa2, 0xef, 0xb0, 0xe0, 0x27, 0xb4, 0x81, 0xde, 0x8f, 0x0c, 0xb4, 0xee, 0xa9, 0xb0, 0x2e, 0xc6, 0x82, 0xd4, 0xcd, 0x3a, 0xac, 0xb6, 0xd6, 0x04, 0x15, 0x30, 0x81, 0x96, 0x9c, 0x1c, 0xa1, 0x8b, 0xda, 0x2b, 0x2d, 0x5c, 0x4e, 0xde, 0xda, 0x4f, 0x85, 0x31, 0x8b, 0xe2, 0x73, 0x46, 0x7f, 0x06, 0xf2, 0x1f, 0xfa, 0xad, 0xe7, 0x63, 0x13, 0x9f, 0xdd, 0x2e, 0x9b, 0xdb, 0xee, 0x0a, 0x4e, 0x18, 0x8b, 0x70, 0x6b, 0x6f, 0x33, 0x63, 0x85, 0x9c, 0x38}

// curl --insecure --show-headers --verbose --http3-only --resolve example.com:8443:127.0.0.1 https://example.com:8443/
var packet3 = []byte{0xc3, 0x00, 0x00, 0x00, 0x01, 0x14, 0x4b, 0x85, 0xb5, 0xef, 0xfc, 0x8e, 0x69, 0x63, 0x6e, 0xfe, 0x9c, 0xbe, 0xa2, 0x55, 0xb0, 0xdc, 0x9f, 0x65, 0x30, 0xfa, 0x14, 0x99, 0xff, 0xeb, 0x9b, 0x83, 0xe0, 0x7d, 0x82, 0xfb, 0x05, 0x48, 0xd0, 0x0d, 0xef, 0x75, 0x84, 0x4c, 0x9e, 0x22, 0xf3, 0x00, 0x80, 0x00, 0x04, 0x7c, 0x1b, 0x4c, 0x27, 0xf2, 0x2a, 0x83, 0x6b, 0x42, 0x30, 0xa3, 0xaa, 0xa0, 0x89, 0x0e, 0x57, 0x5f, 0xa1, 0xd9, 0x38, 0xd6, 0xc1, 0xf3, 0x42, 0xb5, 0x62, 0x53, 0x7a, 0x95, 0x51, 0x71, 0x90, 0xb2, 0xee, 0xe0, 0x81, 0xd6, 0xa6, 0x1a, 0x60, 0x7e, 0xc5, 0x1b, 0xa3, 0xda, 0xaa, 0xe4, 0x9c, 0x00, 0xdd, 0x63, 0x32, 0xa0, 0x91, 0xc0, 0xbb, 0xd6, 0xbb, 0x49, 0x34, 0x52, 0x18, 0x9e, 0x12, 0x4c, 0x4c, 0x0d, 0xd7, 0x49, 0x01, 0x13, 0x89, 0x1d, 0x01, 0xe1, 0x12, 0xa9, 0x79, 0x1b, 0xbb, 0x81, 0xdf, 0x24, 0x1d, 0x4e, 0xfd, 0x97, 0x26, 0x6d, 0x90, 0x3f, 0x7d, 0x21, 0x90, 0x04, 0xc4, 0x7d, 0xfa, 0x1b, 0x60, 0x57, 0x15, 0x74, 0x83, 0xfe, 0x02, 0x3b, 0x2c, 0xf3, 0x1c, 0x20, 0x82, 0xce, 0x30, 0x2e, 0x91, 0x71, 0x75, 0x1a, 0x18, 0x59, 0x44, 0x95, 0xa3, 0x3b, 0xc0, 0x79, 0x8b, 0x38, 0x21, 0xc6, 0x8d, 0x88, 0xe0, 0xc5, 0x75, 0x63, 0x00, 0xd5, 0x6a, 0x86, 0x6e, 0x4a, 0xa0, 0xb6, 0xb2, 0x31, 0x92, 0x7b, 0x92, 0x94, 0xe1, 0x40, 0x6a, 0xfe, 0x76, 0x22, 0xca, 0x60, 0x0a, 0xe3, 0x30, 0x90, 0x62, 0x29, 0x70, 0xad, 0x8d, 0x26, 0x93, 0xd6, 0xfd, 0x06, 0x21, 0x52, 0x01, 0x09, 0xdb, 0x76, 0x69, 0xb9, 0x2e, 0x1a, 0xb5, 0x4d, 0x15, 0x97, 0x06, 0xba, 0x6a, 0xc6, 0x8e, 0x66, 0xda, 0xd6, 0xd7, 0x5c, 0x8d, 0xb6, 0x14, 0x46, 0xed, 0xab, 0x04, 0x52, 0x00, 0xc2, 0xc4, 0xfb, 0x9b, 0x9e, 0x53, 0xd4, 0x16, 0x53, 0x86, 0xa3, 0xcc, 0xe2, 0xec, 0x6b, 0x6d, 0x09, 0x52, 0xe0, 0x0a, 0xf9, 0x03, 0xcd, 0xed, 0xd6, 0x35, 0xd8, 0x50, 0xb6, 0x0a, 0x84, 0x43, 0x79, 0xbd, 0xb4, 0xb5, 0x54, 0xfd, 0x35, 0x0e, 0xff, 0xd9, 0xcb, 0x93, 0xd4, 0x27, 0xa7, 0xa7, 0x58, 0x16, 0x79, 0x18, 0x8d, 0xc3, 0x26, 0x0a, 0xa7, 0xe8, 0x90, 0x84, 0x23, 0xac, 0xa0, 0x01, 0x38, 0xf9, 0x27, 0x21, 0xcc, 0x0c, 0xb2, 0xad, 0x9d, 0x9b, 0x8a, 0x1b, 0xba, 0x4e, 0x6d, 0x82, 0xea, 0x89, 0x37, 0xf9, 0xff, 0x5c, 0xf7, 0x8a, 0x13, 0xbf, 0x49, 0xe4, 0x26, 0xee, 0x92, 0x1d, 0x28, 0x98, 0xe8, 0x4e, 0xe5, 0x9f, 0xd1, 0xf1, 0x1d, 0x6f, 0xed, 0x9b, 0xdc, 0x9f, 0xf1, 0xdd, 0x7b, 0xe9, 0xea, 0xd7, 0x15, 0x79, 0x77, 0x1b, 0x34, 0xa0, 0xc3, 0xce, 0xc1, 0xf1, 0xc0, 0x37, 0x7f, 0x40, 0x24, 0xfe, 0x19, 0x0b, 0xae, 0x83, 0xf9, 0x70, 0xed, 0xd3, 0x24, 0xb0, 0x19, 0xbb, 0x54, 0x2b, 0x21, 0x09, 0x8d, 0x77, 0xac, 0x7d, 0x73, 0x00, 0x71, 0x15, 0x1a, 0x64, 0xe5, 0x04, 0x08, 0x4e, 0x58, 0x13, 0x68, 0xa8, 0x3d, 0xb4, 0x0b, 0x1f, 0x66, 0x8f, 0x34, 0x71, 0x0f, 0x61, 0x61, 0x8c, 0x5c, 0xae, 0x23, 0x22, 0xf7, 0x34, 0x34, 0x2b, 0x8f, 0xd6, 0x36, 0x3d, 0x01, 0xa6, 0x35, 0x7a, 0x21, 0x36, 0xc4, 0x70, 0x12, 0xdf, 0x6d, 0x52, 0x5e, 0x5c, 0x36, 0x2d, 0xe0, 0x9d, 0x0d, 0x60, 0xd8, 0x23, 0xb1, 0xf0, 0x2f, 0x42, 0x58, 0x96, 0xaf, 0x60, 0x70, 0xd5, 0x33, 0xdb, 0x26, 0x21, 0x3b, 0x8d, 0xda, 0x95, 0x97, 0x1c, 0xdd, 0xdc, 0x3c, 0xa8, 0x05, 0xc5, 0x08, 0xac, 0x42, 0xdf, 0x55, 0xba, 0x68, 0x00, 0xf7, 0xdc, 0xd0, 0xba, 0xab, 0xcb, 0x01, 0x88, 0x1a, 0x90, 0x2a, 0x76, 0xf8, 0xb0, 0x66, 0xa5, 0x55, 0x12, 0x96, 0xdb, 0x56, 0x05, 0x05, 0x89, 0x0b, 0x5a, 0xdc, 0xad, 0x0e, 0xa9, 0x93, 0x46, 0xea, 0x34, 0xec, 0x25, 0x4d, 0xff, 0xba, 0x3e, 0x2b, 0x38, 0xfe, 0x06, 0x79, 0xb0, 0xca, 0xf1, 0xe4, 0xe7, 0x3f, 0x0f, 0x00, 0xc3, 0x9e, 0x0a, 0x7a, 0x4f, 0xaa, 0x55, 0x3c, 0xe8, 0xe1, 0x3b, 0x16, 0x3b, 0xc0, 0xd9, 0x73, 0xbb, 0x7e, 0x07, 0x7d, 0xc0, 0x01, 0xaf, 0x07, 0x7e, 0xef, 0xec, 0x96, 0xf3, 0xb6, 0x59, 0x50, 0xcc, 0xcd, 0xbe, 0x23, 0x82, 0xf3, 0xa2, 0x4e, 0x55, 0xb7, 0x87, 0xf6, 0x24, 0x47, 0xed, 0x3c, 0x9f, 0x95, 0xf8, 0x7a, 0x0c, 0x42, 0x91, 0x50, 0x5f, 0x6b, 0x17, 0xc8, 0x80, 0xb9, 0x98, 0x2e, 0x38, 0xf2, 0xbe, 0xe6, 0x50, 0xe4, 0xfa, 0x5d, 0x23, 0xa1, 0x25, 0xd6, 0x7f, 0x7a, 0x7e, 0x5e, 0x1d, 0x96, 0x5b, 0xb3, 0xf3, 0x5b, 0xd5, 0xb9, 0xa8, 0x13, 0x8f, 0xcb, 0xad, 0x45, 0xb4, 0x99, 0x1b, 0x6b, 0x87, 0xae, 0x36, 0x18, 0xad, 0x22, 0x9d, 0xd4, 0x29, 0x2c, 0x52, 0x95, 0x38, 0x40, 0x4d, 0xe6, 0x89, 0xff, 0x68, 0xa9, 0xd6, 0xe1, 0xc0, 0xe5, 0x03, 0x6a, 0x10, 0x42, 0xa8, 0x21, 0x4b, 0x20, 0x43, 0x48, 0x80, 0x7b, 0x56, 0xda, 0x5c, 0xe8, 0xce, 0xdf, 0xe8, 0x20, 0x8a, 0x5b, 0x12, 0xd7, 0x47, 0x0d, 0x20, 0xbe, 0xa0, 0xdb, 0xb8, 0xb4, 0x30, 0x0f, 0x99, 0x4a, 0xb5, 0x88, 0x7e, 0x80, 0x1d, 0xa1, 0xfc, 0x61, 0x70, 0xf0, 0x6d, 0x0d, 0x58, 0x48, 0xc5, 0xc0, 0x21, 0xa4, 0x89, 0x15, 0xe2, 0xcb, 0x9b, 0x70, 0x7e, 0x8b, 0x6f, 0x0f, 0xcd, 0x18, 0xfb, 0x78, 0x1b, 0x24, 0x3a, 0x16, 0x3c, 0xfa, 0x0f, 0xdd, 0x28, 0xe3, 0x2a, 0xe4, 0x39, 0x81, 0xa4, 0x4a, 0x38, 0x60, 0x00, 0x41, 0x95, 0xab, 0x7b, 0xf0, 0xa2, 0xbe, 0xe4, 0x26, 0xb5, 0x4b, 0x0b, 0x11, 0x15, 0x18, 0x18, 0x90, 0x30, 0x70, 0x20, 0xd0, 0x85, 0x41, 0x95, 0x20, 0x20, 0xcf, 0x95, 0xe1, 0x8c, 0xac, 0x6f, 0x15, 0xa1, 0x30, 0xfd, 0x32, 0xc3, 0xcf, 0xe1, 0x49, 0x90, 0x34, 0x64, 0xe0, 0xcd, 0xf8, 0x97, 0x11, 0xdf, 0x17, 0xd3, 0xd5, 0x2c, 0xa5, 0x2e, 0x6f, 0xf0, 0x95, 0xeb, 0xda, 0x7c, 0xbb, 0x2d, 0x3e, 0x4f, 0x35, 0x9a, 0x66, 0xf0, 0x3e, 0x3f, 0x8a, 0x6e, 0x3e, 0xaf, 0x0f, 0xcb, 0x4f, 0xaf, 0xb9, 0xfb, 0x00, 0x5c, 0x50, 0xc5, 0x90, 0xc3, 0x34, 0x8f, 0xe3, 0xa4, 0xb4, 0xe3, 0xec, 0xfa, 0xc3, 0x52, 0x7c, 0x8a, 0xf5, 0x08, 0x36, 0x83, 0x23, 0xfa, 0xd2, 0xa2, 0x3b, 0x35, 0x2a, 0xea, 0x1f, 0xd9, 0xe8, 0x54, 0x6d, 0x88, 0x5c, 0xe6, 0xf4, 0x4c, 0x8e, 0x6d, 0xc8, 0x04, 0x98, 0xf9, 0x74, 0xaf, 0x99, 0x90, 0x12, 0x9d, 0x90, 0xf8, 0xa0, 0x29, 0x3d, 0xfd, 0x9e, 0xab, 0x0f, 0xda, 0x1d, 0x61, 0xe0, 0xa0, 0xc8, 0xc3, 0xfd, 0xd0, 0xe6, 0xb8, 0x0b, 0x44, 0x9c, 0x4c, 0x31, 0x14, 0xe9, 0x31, 0xf8, 0x2d, 0x54, 0x5b, 0x16, 0x3d, 0x0c, 0xe1, 0x20, 0x12, 0x44, 0x4c, 0xac, 0x0c, 0x93, 0x9f, 0x02, 0xee, 0x89, 0x13, 0xcb, 0x99, 0xec, 0x70, 0xeb, 0xa7, 0xd8, 0xfc, 0xed, 0xf5, 0xae, 0xfd, 0xc7, 0x27, 0xda, 0x6b, 0x25, 0x32, 0x56, 0xbc, 0x39, 0x98, 0xbf, 0x5e, 0xd0, 0xb7, 0xe1, 0x0a, 0x66, 0x2c, 0x34, 0x9b, 0xad, 0xb6, 0x99, 0x1b, 0xa0, 0x01, 0xa2, 0x8c, 0xfc, 0xe1, 0xfb, 0xa9, 0xf2, 0xf9, 0xae, 0x32, 0xfa, 0x5c, 0x1c, 0x9c, 0xc6, 0xa3, 0x86, 0x34, 0xee, 0x23, 0x1f, 0x94, 0xe4, 0x07, 0x5d, 0x92, 0xd7, 0xb1, 0x58, 0x66, 0x04, 0x3e, 0xb4, 0xad, 0x08, 0x64, 0x10, 0xac, 0x92, 0x28, 0xa6, 0xa5, 0x23, 0x70, 0x12, 0x23, 0xbf, 0x0e, 0xa8, 0x1d, 0x34, 0x40, 0xc8, 0xd1, 0x2e, 0xe2, 0x06, 0x59, 0x75, 0xec, 0x27, 0x17, 0x0f, 0x56, 0xe0, 0x3d, 0x72, 0x9f, 0x6e, 0xc1, 0x11, 0x1b, 0x88, 0xe1, 0xcf, 0x5f, 0xed, 0xac, 0x2f, 0x98, 0x7f, 0x2c, 0x3b, 0x6e, 0x31, 0x1c, 0x9f, 0xbf, 0xe7, 0x0c, 0x8a, 0x3d, 0x49, 0x31, 0xd0, 0x57, 0x5f, 0x6a, 0x70, 0x49, 0x30, 0xd3, 0x9e, 0x71, 0xc0, 0x90, 0xfa, 0xcb, 0xd4, 0x6a, 0x70, 0x95, 0x5b, 0xfb, 0xb8, 0x90, 0xd3, 0xc6, 0x92, 0x1e, 0xed, 0x69, 0xcd, 0x99, 0x02, 0x19, 0x5f, 0x59, 0x3e, 0xb9, 0x92, 0x07, 0xf7, 0xfe, 0xd5, 0x34, 0x05, 0xcb, 0x3e, 0xa3, 0xb5, 0x5d, 0x8e, 0x45, 0x2e, 0x51, 0xfb, 0x98, 0x0f, 0xaf, 0xa9, 0xe9, 0xfc, 0x88, 0xd2, 0x7f, 0x88, 0x91, 0xef, 0x08, 0xf4, 0xec, 0x93, 0x11, 0x1d, 0x27, 0x04, 0x2a, 0xcb, 0xde, 0x2d, 0x37, 0x95, 0x24, 0x9f, 0x93, 0x2a, 0xfa, 0x4e, 0x1f, 0xd8, 0x17, 0xa5, 0xd2, 0x09, 0x19, 0x3f, 0x37, 0xcc, 0x76, 0x27, 0x12, 0x3e, 0xe4, 0x87, 0x06, 0xe7, 0xc1, 0xd0, 0x82, 0x02, 0xe8, 0x79, 0xf0, 0x20, 0xb9, 0x30, 0xab, 0xc8, 0xa7, 0x31, 0x74, 0x7c, 0x08, 0x0c}
