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

package store

import (
	"encoding/binary"
	"hash"
	"math/bits"
)

// sm3 implements the SM3 cryptographic hash function (GB/T 32905-2016).
// SM3 produces a 256-bit (32-byte) hash value.
// This implementation follows the Chinese national standard.

const (
	sm3Size      = 32
	sm3BlockSize = 64
)

// sm3Iv are the initial hash values (IV) for SM3.
var sm3Iv = [8]uint32{
	0x7380166f, 0x4914b2b9, 0x172442d7, 0xda8a0600,
	0xa96f30bc, 0x163138aa, 0xe38dee4d, 0xb0fb0e4e,
}

func sm3Tj(j int) uint32 {
	if j <= 15 {
		return 0x79cc4519
	}
	return 0x7a879d8a
}

func sm3FF(x, y, z uint32, j int) uint32 {
	if j <= 15 {
		return x ^ y ^ z
	}
	return (x & y) | (x & z) | (y & z)
}

func sm3GG(x, y, z uint32, j int) uint32 {
	if j <= 15 {
		return x ^ y ^ z
	}
	return (x & y) | (^x & z)
}

func sm3P0(x uint32) uint32 {
	return x ^ bits.RotateLeft32(x, 9) ^ bits.RotateLeft32(x, 17)
}

func sm3P1(x uint32) uint32 {
	return x ^ bits.RotateLeft32(x, 15) ^ bits.RotateLeft32(x, 23)
}

type sm3Digest struct {
	h   [8]uint32
	x   [sm3BlockSize]byte
	nx  int
	len uint64 // total message length in BYTES
}

// NewSM3 returns a new hash.Hash computing the SM3 checksum.
func NewSM3() hash.Hash {
	d := new(sm3Digest)
	d.Reset()
	return d
}

func (d *sm3Digest) Reset() {
	d.h = sm3Iv
	d.nx = 0
	d.len = 0
}

func (d *sm3Digest) Size() int      { return sm3Size }
func (d *sm3Digest) BlockSize() int { return sm3BlockSize }

func (d *sm3Digest) Write(p []byte) (n int, err error) {
	n = len(p)
	d.len += uint64(n)

	nn := d.nx
	if nn > 0 {
		k := sm3BlockSize - nn
		if k > len(p) {
			k = len(p)
		}
		copy(d.x[nn:], p[:k])
		d.nx = nn + k
		if d.nx == sm3BlockSize {
			d.block(d.x[:])
			d.nx = 0
		}
		p = p[k:]
	}

	for len(p) >= sm3BlockSize {
		d.block(p[:sm3BlockSize])
		p = p[sm3BlockSize:]
	}

	if len(p) > 0 {
		copy(d.x[:], p)
		d.nx = len(p)
	}
	return
}

func (d *sm3Digest) Sum(in []byte) []byte {
	d0 := *d
	hash := d0.checkSum()
	return append(in, hash[:]...)
}

// checkSum computes the final SM3 hash after padding (Merkle-Damgård padding).
// Padding: append 1 bit (0x80), then pad with 0 bits until length mod 512 == 448,
// then append 64-bit message length in BITS (big-endian).
func (d *sm3Digest) checkSum() [32]byte {
	// len in BITS
	bitLen := d.len * 8

	// Append 0x80
	d.x[d.nx] = 0x80
	d.nx++

	// If the remaining space in the block is less than 8 bytes (can't fit length),
	// fill this block with zeros and process it
	if d.nx > sm3BlockSize-8 {
		for d.nx < sm3BlockSize {
			d.x[d.nx] = 0
			d.nx++
		}
		d.block(d.x[:])
		d.nx = 0
	}

	// Pad with zeros until position 56 (8 bytes before end)
	for d.nx < sm3BlockSize-8 {
		d.x[d.nx] = 0
		d.nx++
	}

	// Append length in bits (big-endian, 64-bit)
	binary.BigEndian.PutUint64(d.x[56:], bitLen)

	// Process the final block
	d.block(d.x[:])

	var digest [32]byte
	for i := 0; i < 8; i++ {
		binary.BigEndian.PutUint32(digest[i*4:], d.h[i])
	}
	return digest
}

// block processes a 64-byte message block.
func (d *sm3Digest) block(p []byte) {
	var w [68]uint32
	var wp [64]uint32

	// W0..W15
	for i := 0; i < 16; i++ {
		w[i] = binary.BigEndian.Uint32(p[i*4:])
	}

	// W16..W67
	for j := 16; j < 68; j++ {
		w[j] = sm3P1(w[j-16]^w[j-9]^bits.RotateLeft32(w[j-3], 15)) ^ bits.RotateLeft32(w[j-13], 7) ^ w[j-6]
	}

	// W'0..W'63
	for j := 0; j < 64; j++ {
		wp[j] = w[j] ^ w[j+4]
	}

	a, b, c, dval, e, f, g, h := d.h[0], d.h[1], d.h[2], d.h[3], d.h[4], d.h[5], d.h[6], d.h[7]

	for j := 0; j < 64; j++ {
		ss1 := bits.RotateLeft32(bits.RotateLeft32(a, 12)+e+bits.RotateLeft32(sm3Tj(j), j), 7)
		ss2 := ss1 ^ bits.RotateLeft32(a, 12)
		tt1 := sm3FF(a, b, c, j) + dval + ss2 + wp[j]
		tt2 := sm3GG(e, f, g, j) + h + ss1 + w[j]
		dval = c
		c = bits.RotateLeft32(b, 9)
		b = a
		a = tt1
		h = g
		g = bits.RotateLeft32(f, 19)
		f = e
		e = sm3P0(tt2)
	}

	d.h[0] ^= a
	d.h[1] ^= b
	d.h[2] ^= c
	d.h[3] ^= dval
	d.h[4] ^= e
	d.h[5] ^= f
	d.h[6] ^= g
	d.h[7] ^= h
}

// Ensure sm3Digest implements hash.Hash at compile time.
var _ hash.Hash = (*sm3Digest)(nil)
