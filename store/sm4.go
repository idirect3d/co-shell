// Author: L.Shuang
// Created: 2026-07-06
// Last Modified: 2026-07-06
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package store provides SM4 cipher implementation.
// SM4 is a block cipher defined in GB/T 32907-2016, with 128-bit key and 128-bit block size.
// This implementation follows the Chinese national standard.

package store

import (
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
)

// SM4 block size in bytes.
const sm4BlockSize = 16

// sm4Sbox is the SM4 S-box (Table 1 in GB/T 32907-2016).
var sm4Sbox = [256]uint8{
	0xd6, 0x90, 0xe9, 0xfe, 0xcc, 0xe1, 0x3d, 0xb7, 0x16, 0xb6, 0x14, 0xc2, 0x28, 0xfb, 0x2c, 0x05,
	0x2b, 0x67, 0x9a, 0x76, 0x2a, 0xbe, 0x04, 0xc3, 0xaa, 0x44, 0x13, 0x26, 0x49, 0x86, 0x06, 0x99,
	0x9c, 0x42, 0x50, 0xf4, 0x91, 0xef, 0x98, 0x7a, 0x33, 0x54, 0x0b, 0x43, 0xed, 0xcf, 0xac, 0x62,
	0xe4, 0xb3, 0x1c, 0xa9, 0xc9, 0x08, 0xe8, 0x95, 0x80, 0xdf, 0x94, 0xfa, 0x75, 0x8f, 0x3f, 0xa6,
	0x47, 0x07, 0xa7, 0xfc, 0xf3, 0x73, 0x17, 0xba, 0x83, 0x59, 0x3c, 0x19, 0xe6, 0x85, 0x4f, 0xa8,
	0x68, 0x6b, 0x81, 0xb2, 0x71, 0x64, 0xda, 0x8b, 0xf8, 0xeb, 0x0f, 0x4b, 0x70, 0x56, 0x9d, 0x35,
	0x1e, 0x24, 0x0e, 0x5e, 0x63, 0x58, 0xd1, 0xa2, 0x25, 0x22, 0x7c, 0x3b, 0x01, 0x21, 0x78, 0x87,
	0xd4, 0x00, 0x46, 0x57, 0x9f, 0xd3, 0x27, 0x52, 0x4c, 0x36, 0x02, 0xe7, 0xa0, 0xc4, 0xc8, 0x9e,
	0xea, 0xbf, 0x8a, 0xd2, 0x40, 0xc7, 0x38, 0xb5, 0xa3, 0xf7, 0xf2, 0xce, 0xf9, 0x61, 0x15, 0xa1,
	0xe0, 0xae, 0x5d, 0xa4, 0x9b, 0x34, 0x1a, 0x55, 0xad, 0x93, 0x32, 0x30, 0xf5, 0x8c, 0xb1, 0xe3,
	0x1d, 0xf6, 0xe2, 0x2e, 0x82, 0x66, 0xca, 0x60, 0xc0, 0x29, 0x23, 0xab, 0x0d, 0x53, 0x4e, 0x6f,
	0xd5, 0xdb, 0x37, 0x45, 0xde, 0xfd, 0x8e, 0x2f, 0x03, 0xff, 0x6a, 0x72, 0x6d, 0x6c, 0x5b, 0x51,
	0x8d, 0x1b, 0xaf, 0x92, 0xbb, 0xdd, 0xbc, 0x7f, 0x11, 0xd9, 0x5c, 0x41, 0x1f, 0x10, 0x5a, 0xd8,
	0x0a, 0xc1, 0x31, 0x88, 0xa5, 0xcd, 0x7b, 0xbd, 0x2d, 0x74, 0xd0, 0x12, 0xb8, 0xe5, 0xb4, 0xb0,
	0x89, 0x69, 0x97, 0x4a, 0x0c, 0x96, 0x77, 0x7e, 0x65, 0xb9, 0xf1, 0x09, 0xc5, 0x6e, 0xc6, 0x84,
	0x18, 0xf0, 0x7d, 0xec, 0x3a, 0xdc, 0x4d, 0x20, 0x79, 0xee, 0x5f, 0x3e, 0xd7, 0xcb, 0x39, 0x48,
}

// sm4Fk are the system parameters (fixed constants) used in key expansion.
var sm4Fk = [4]uint32{0xa3b1bac6, 0x56aa3350, 0x677d9197, 0xb27022dc}

// sm4Ck are the fixed constants used in key expansion (32 elements).
var sm4Ck = [32]uint32{
	0x00070e15, 0x1c232a31, 0x383f464d, 0x545b6269,
	0x70777e85, 0x8c939aa1, 0xa8afb6bd, 0xc4cbd2d9,
	0xe0e7eef5, 0xfc030a11, 0x181f262d, 0x343b4249,
	0x50575e65, 0x6c737a81, 0x888f969d, 0xa4abb2b9,
	0xc0c7ced5, 0xdce3eaf1, 0xf8ff060d, 0x141b2229,
	0x30373e45, 0x4c535a61, 0x686f767d, 0x848b9299,
	0xa0a7aeb5, 0xbcc3cad1, 0xd8dfe6ed, 0xf4fb0209,
	0x10171e25, 0x2c333a41, 0x484f565d, 0x646b7279,
}

// sm4Cipher implements the SM4 block cipher (cipher.Block interface).
type sm4Cipher struct {
	rk [32]uint32 // round keys
}

// NewSM4Cipher creates a new SM4 cipher with the given key (must be 16 bytes).
func NewSM4Cipher(key []byte) (cipher.Block, error) {
	if len(key) != 16 {
		return nil, errors.New("sm4: key must be 16 bytes")
	}
	c := &sm4Cipher{}
	c.keyExpansion(key)
	return c, nil
}

func (c *sm4Cipher) BlockSize() int { return sm4BlockSize }

func (c *sm4Cipher) Encrypt(dst, src []byte) {
	if len(src) < sm4BlockSize {
		panic("sm4: src too short")
	}
	if len(dst) < sm4BlockSize {
		panic("sm4: dst too short")
	}

	x := bytesToWords(src) // x[0..3]=plaintext
	for i := 0; i < 32; i++ {
		x[i+4] = x[i] ^ sm4L(sm4Tau(x[i+1]^x[i+2]^x[i+3]^c.rk[i]))
	}
	// Reverse final ordering: (x35, x34, x33, x32)
	wordsToBytes([]uint32{x[35], x[34], x[33], x[32]}, dst)
}

func (c *sm4Cipher) Decrypt(dst, src []byte) {
	if len(src) < sm4BlockSize {
		panic("sm4: src too short")
	}
	if len(dst) < sm4BlockSize {
		panic("sm4: dst too short")
	}

	x := bytesToWords(src) // x[0..3]=ciphertext
	// Decryption uses round keys in reverse order
	for i := 0; i < 32; i++ {
		x[i+4] = x[i] ^ sm4L(sm4Tau(x[i+1]^x[i+2]^x[i+3]^c.rk[31-i]))
	}
	wordsToBytes([]uint32{x[35], x[34], x[33], x[32]}, dst)
}

// keyExpansion generates round keys from the 128-bit master key.
func (c *sm4Cipher) keyExpansion(key []byte) {
	mk := bytesToWords(key)
	k := [36]uint32{}
	for i := 0; i < 4; i++ {
		k[i] = mk[i] ^ sm4Fk[i]
	}
	for i := 0; i < 32; i++ {
		c.rk[i] = k[i] ^ sm4LPrime(sm4Tau(k[i+1]^k[i+2]^k[i+3]^sm4Ck[i]))
		k[i+4] = c.rk[i]
	}
}

// sm4Tau is the non-linear transformation: applies S-box to each byte.
func sm4Tau(input uint32) uint32 {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, input)
	return binary.BigEndian.Uint32([]byte{
		sm4Sbox[b[0]],
		sm4Sbox[b[1]],
		sm4Sbox[b[2]],
		sm4Sbox[b[3]],
	})
}

// sm4L is the linear transformation L: B ^ (B<<<2) ^ (B<<<10) ^ (B<<<18) ^ (B<<<24).
func sm4L(b uint32) uint32 {
	return b ^ rotl32(b, 2) ^ rotl32(b, 10) ^ rotl32(b, 18) ^ rotl32(b, 24)
}

// sm4LPrime is the simplified linear transformation L': B ^ (B<<<13) ^ (B<<<23).
func sm4LPrime(b uint32) uint32 {
	return b ^ rotl32(b, 13) ^ rotl32(b, 23)
}

// rotl32 rotates a 32-bit value left by n bits.
func rotl32(x uint32, n uint) uint32 {
	return (x << n) | (x >> (32 - n))
}

// bytesToWords converts a 16-byte slice to a 4-element uint32 array.
func bytesToWords(b []byte) [36]uint32 {
	var x [36]uint32
	for i := 0; i < 4; i++ {
		x[i] = binary.BigEndian.Uint32(b[i*4:])
	}
	return x
}

// wordsToBytes converts 4 uint32 words to a 16-byte slice.
func wordsToBytes(w []uint32, dst []byte) {
	for i := 0; i < 4; i++ {
		binary.BigEndian.PutUint32(dst[i*4:], w[i])
	}
}

// Ensure sm4Cipher implements cipher.Block at compile time.
var _ cipher.Block = (*sm4Cipher)(nil)

// NewSM4GCM returns a cipher.AEAD that uses SM4 as the underlying block cipher
// for GCM mode. This is used by store/vault.go's encryptField/decryptField.
// The returned AEAD uses Go's standard GCM implementation with SM4 as the block cipher.
//
// Note: GCM mode is not part of the SM4 standard (GB/T 32907-2016), but using
// SM4 + GCM provides authenticated encryption, which is a standard practice
// for combining SM4 with a well-studied AEAD mode.
func NewSM4GCM(key []byte) (cipher.AEAD, error) {
	block, err := NewSM4Cipher(key)
	if err != nil {
		return nil, fmt.Errorf("sm4-gcm: %w", err)
	}
	return cipher.NewGCM(block)
}
