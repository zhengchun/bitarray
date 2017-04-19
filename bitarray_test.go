package bitarray

import (
	"reflect"
	"testing"
)

func TestSetGet(t *testing.T) {
	data := map[uint32]bool{
		0:          true,
		1:          true,
		2:          true,
		3:          false,
		4:          true,
		100:        false,
		10000:      false,
		100000:     true,
		1000000:    true,
		4294967295: true,
		4294967294: true,
		4294967293: false,
		101:        true,
	}
	bitset := New()
	// Set
	for index, val := range data {
		bitset.Set(index, val)
	}
	// Test
	for index, val := range data {
		b := bitset.Get(index)
		if b != val {
			t.Fatal("expected index value is %v but got %v", val, b)
		}
	}
}

func TestAnd(t *testing.T) {
	b1 := New()
	count := 10
	for i := 0; i < count; i++ {
		b1.Set(uint32(i), true)
	}
	b2 := New()
	for i := 5; i < count; i++ {
		b2.Set(uint32(i), true)
	}
	x := b1.And(b2)
	bits := x.GetBitIndexes() // 5,6,7,8,9
	if !reflect.DeepEqual(bits, []uint32{5, 6, 7, 8, 9}) {
		t.Fatal("expected index list is [5,6,7,8,9],but got %v", bits)
	}
}

func TestOr(t *testing.T) {
	b1 := New()
	count := 10
	for i := 0; i < count; i++ {
		b1.Set(uint32(i), true)
	}
	b2 := New()
	for i := 5; i < count; i++ {
		b2.Set(uint32(i), false)
	}
	x := b1.Or(b2)
	bits := x.GetBitIndexes()
	if !reflect.DeepEqual(bits, []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}) {
		t.Fatal("expected index list is [0, 1, 2, 3, 4, 5, 6, 7, 8, 9],but got %v", bits)
	}
}

func TestNot(t *testing.T) {
	b1 := New()
	count := 31
	for i := 0; i < count; i++ {
		b1.Set(uint32(i), true)
	}
	x := b1.Not(31) //
	bits := x.GetBitIndexes()
	if !reflect.DeepEqual(bits, []uint32{31}) {
		t.Fatal("expected index list is [31],but got %v", bits)
	}
}

func TestXor(t *testing.T) {
	b1 := New()
	count := 36
	for i := 0; i < count; i++ {
		b1.Set(uint32(i), true)
	}
	bits, typ := b1.GetCompressed()
	b2 := Create(typ, bits)
	x := b1.Xor(b2)
	if l := countOnes(x); l != 0 {
		t.Fatalf("countOnes(x)!=0")
	}
}

func countOnes(s *BitArray) int {
	BitCount := func(n uint32) uint32 {
		n -= ((n >> 1) & 0x55555555)
		n = (((n >> 2) & 0x33333333) + (n & 0x33333333))
		n = (((n >> 4) + n) & 0x0f0f0f0f)
		return ((n * 0x01010101) >> 24)
	}
	if s.state == IndexesType {
		return len(s.offsets)
	}
	c := 0
	s.checkBitArray()
	for _, i := range s.uncompressed {
		c += int(BitCount(i))
	}
	return int(c)
}

func countZeros(s *BitArray) int {
	if s.state == IndexesType {
		ones := len(s.offsets)
		k := s.getOffsets()
		l := k[len(k)-1]
		return int(l) - ones
	}

	s.checkBitArray()
	count := len(s.uncompressed) << 5
	cc := countOnes(s)
	return count - cc
}
