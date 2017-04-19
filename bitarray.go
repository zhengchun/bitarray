/*
Package bitarray provides an object type which efficiently represents an array of booleans.
*/
package bitarray

import (
	"sync"
)

type Type int

const (
	BitArrayType Type = iota
	WAHType
	IndexesType
)

// BitArray is an array data structure that compactly stores bits.
type BitArray struct {
	mux          sync.Mutex
	state        Type
	curMax       uint32
	isDirty      bool
	offsets      map[uint32]bool
	compressed   []uint32
	uncompressed []uint32
}

func get(s *BitArray, index uint32) bool {
	pointer := index >> 5
	mask := 1 << (31 - (index % 32))
	if int(pointer) < len(s.uncompressed) {
		return (s.uncompressed[pointer] & uint32(mask)) != 0
	}
	return false
}

func set(s *BitArray, index uint32, val bool) {
	s.isDirty = true
	pointer := index >> 5
	mask := 1 << (31 - (index % 32))
	if val {
		s.uncompressed[pointer] |= uint32(mask)
	} else {
		s.uncompressed[pointer] &= ^uint32(mask)
	}
}

func write31Bits(list []uint32, index int32, val uint32) []uint32 {
	list = resizeAsNeeded(list, index+32)
	var (
		off     = index % 32
		pointer = index >> 5
	)
	if int(pointer) >= len(list)-1 {
		list = append(list, 0)
	}

	l := uint64(list[pointer])<<32 + uint64(list[pointer+1])
	l |= uint64(val) << uint64((33 - off))

	list[pointer] = uint32(l >> 32)
	list[pointer+1] = uint32(l & 0xffffffff)
	return list
}

func writeOnes(list []uint32, index int32, count uint32) []uint32 {
	list = resizeAsNeeded(list, index)
	var (
		off     = index % 32
		pointer = index >> 5
		ccount  = int32(count)
		indx    = index
		x       = 32 - off
	)

	if int(pointer) >= len(list) {
		list = append(list, 0)
	}

	if ccount > x || x == 32 {
		list[pointer] |= 0xffffffff >> uint32(off)
		ccount -= x
		indx += x
	} else {
		list[pointer] |= (0xffffffff << uint32(ccount)) >> uint32(off)
		ccount = 0
	}

	checklast := true
	for ccount >= 32 {
		if checklast && list[len(list)-1] == 0 {
			list = list[:len(list)-1]
			checklast = false
		}

		list = append(list, 0xffffffff)
		ccount -= 32
		indx += 32
	}
	p := indx >> 5
	off = indx % 32
	if ccount > 0 {
		i := 0xffffffff << uint32(32-ccount)
		if int(p) > len(list)-1 {
			list = append(list, uint32(i))
		} else {
			list[p] |= uint32(i) >> uint32(off)
		}
	}
	return list
}

func take31Bits(data []uint32, index uint32) uint32 {
	var l1, l2, l, ret uint64
	off := index % 32
	pointer := index >> 5
	l1 = uint64(data[pointer])
	pointer++
	if int(pointer) < len(data) {
		l2 = uint64(data[pointer])
	}
	l = (l1 << 32) + l2
	ret = (l >> (33 - off)) & 0x7fffffff
	return uint32(ret)
}

func flushOnes(compressed []uint32, ones *uint32) []uint32 {
	if *ones > uint32(0) {
		n := 0xc0000000 + *ones
		*ones = 0
		compressed = append(compressed, n)
	}
	return compressed
}

func flushZeros(compressed []uint32, zeros *uint32) []uint32 {
	if *zeros > uint32(0) {
		n := 0x80000000 + *zeros
		*zeros = 0
		compressed = append(compressed, n)
	}
	return compressed
}

func (s *BitArray) uncompress() {
	var (
		index int32
		list  []uint32
	)
	if len(s.compressed) == 0 {
		return
	}

	for _, ci := range s.compressed {
		if ci&0x80000000 == 0 {
			list = write31Bits(list, index, ci)
			index += 31
		} else {
			count := ci & 0x3fffffff
			if ci&0x40000000 > 0 {
				list = writeOnes(list, index, count)
			}
			index += int32(count)
		}
	}
	list = resizeAsNeeded(list, index)
	s.uncompressed = list
}

func (s *BitArray) compress(data []uint32) {
	var (
		compressed []uint32
		zeros      = uint32(0)
		ones       = uint32(0)
		count      = len(data) << 5
	)
	for i := 0; i < count; {
		num := take31Bits(data, uint32(i))
		i += 31
		if num == 0 { // all zero
			zeros += 31
			compressed = flushOnes(compressed, &ones)
		} else if num == 0x7fffffff { // all ones
			ones += 31
			compressed = flushZeros(compressed, &zeros)
		} else { // literal
			compressed = flushOnes(compressed, &ones)
			compressed = flushZeros(compressed, &zeros)
			compressed = append(compressed, num)
		}
	}
	compressed = flushOnes(compressed, &ones)
	compressed = flushZeros(compressed, &zeros)
	s.compressed = compressed
}

func resizeAsNeeded(list []uint32, index int32) []uint32 {
	count := index >> 5
	if len(list) >= int(count) {
		return list
	}
	list2 := make([]uint32, count)
	copy(list2, list)
	return list2
}

func (s *BitArray) resize(index uint32) {
	if s.state == IndexesType {
		return
	}
	c := index >> 5
	c++
	if len(s.uncompressed) == 0 {
		s.uncompressed = make([]uint32, c)
		return
	}
	if int(c) > len(s.uncompressed) {
		ar := make([]uint32, c)
		copy(ar, s.uncompressed)
		s.uncompressed = ar
	}
}

func (s *BitArray) changeTypeIfNeeded() {
	if s.state != IndexesType {
		return
	}

	const BitmapOffsetSwitchOverCount = 10
	t := (s.curMax >> 5) + 1
	c := len(s.offsets)
	if c > int(t) && c > BitmapOffsetSwitchOverCount {
		s.state = BitArrayType
		s.uncompressed = s.uncompressed[:0]
		for i, _ := range s.offsets {
			s.set(i, true)
		}
		s.offsets = make(map[uint32]bool)
	}
}

func (s *BitArray) checkBitArray() {
	switch s.state {
	case BitArrayType:
		return
	case WAHType:
		s.uncompressed = s.uncompressed[:0]
		s.uncompress()
		s.state = BitArrayType
		s.compressed = s.compressed[:0]
	}
}

func (s *BitArray) getOffsets() []uint32 {
	var (
		k = make([]uint32, len(s.offsets))
		i = 0
	)
	for key, _ := range s.offsets {
		k[i] = key
		i++
	}
	return k
}

func (s *BitArray) get(index uint32) bool {
	if s.state == IndexesType {
		if b, ok := s.offsets[index]; ok {
			return b
		}
		return false
	}
	s.checkBitArray()
	s.resize(index)
	return get(s, index)
}

func (s *BitArray) set(index uint32, val bool) {
	if s.state == IndexesType {
		s.isDirty = true
		if val == true {
			s.offsets[index] = true
			if index > s.curMax {
				s.curMax = index
			}
		} else {
			delete(s.offsets, index)
		}
		s.changeTypeIfNeeded()
		return
	}
	s.checkBitArray()
	s.resize(index)
	set(s, index, val)
}

func (s *BitArray) getBitArray() []uint32 {
	if s.state == IndexesType {
		return s.unpackOffsets()
	}
	s.checkBitArray()
	ui := make([]uint32, len(s.uncompressed))
	copy(ui, s.uncompressed)
	return ui
}

func (s *BitArray) unpackOffsets() []uint32 {
	if len(s.offsets) == 0 {
		return []uint32{}
	}
	var max uint32
	k := s.getOffsets()
	max = k[len(k)-1]
	ints := make([]uint32, (max>>5)+1)

	for _, index := range k {
		pointer := index >> 5
		mask := 1 << (31 - (index % 32))
		ints[pointer] |= uint32(mask)
	}
	return ints
}

func (s *BitArray) prelogic(op *BitArray) (left, right []uint32) {
	s.checkBitArray()
	left = s.getBitArray()
	right = op.getBitArray()
	ic, uc := len(left), len(right)
	if ic > uc {
		ar := make([]uint32, ic)
		copy(ar, right)
		right = ar
	} else if ic < uc {
		ar := make([]uint32, uc)
		copy(ar, left)
		left = ar
	}
	return
}

// And performs the bitwise AND operation.
func (s *BitArray) And(op *BitArray) *BitArray {
	s.mux.Lock()
	defer s.mux.Unlock()
	left, right := s.prelogic(op)
	for i := 0; i < len(left); i++ {
		left[i] &= right[i]
	}
	return Create(BitArrayType, left)
}

// Or performs the bitwise OR operation.
func (s *BitArray) Or(op *BitArray) *BitArray {
	s.mux.Lock()
	defer s.mux.Unlock()
	left, right := s.prelogic(op)
	for i := 0; i < len(left); i++ {
		left[i] |= right[i]
	}
	return Create(BitArrayType, left)
}

// Not inverts all the bit values.
func (s *BitArray) Not(size int) *BitArray {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.checkBitArray()
	var (
		left = s.getBitArray()
		c    = len(left)
		ms   = size >> 5
	)
	if size-(ms<<5) > 0 {
		ms++
	}
	if ms > c {
		a := make([]uint32, ms)
		copy(a, left[:c])
		left = a
		c = ms
	}

	for i := 0; i < c; i++ {
		left[i] = ^left[i]
	}
	return Create(BitArrayType, left)
}

// Xor performs the bitwise exclusive OR operation.
func (s *BitArray) Xor(op *BitArray) *BitArray {
	s.mux.Lock()
	defer s.mux.Unlock()
	left, right := s.prelogic(op)
	for i := 0; i < len(left); i++ {
		left[i] ^= right[i]
	}
	return Create(BitArrayType, left)
}

// FreeMemory compresses all the bit values and free used memory.
func (s *BitArray) FreeMemory() {
	if s.state == BitArrayType {
		if len(s.uncompressed) > 0 {
			s.compress(s.uncompressed)
			s.uncompressed = s.uncompressed[:0]
			s.state = WAHType
		}
	}
}

// GetBitIndexes returns all indexe list that index bit value is true.
func (s *BitArray) GetBitIndexes() []uint32 {
	if s.state == IndexesType {
		return s.getBitArray()
	}
	s.checkBitArray()
	var list []uint32
	count := len(s.uncompressed)
	for i := 0; i < count; i++ {
		if s.uncompressed[i] > 0 {
			for j := 0; j < 32; j++ {
				if s.get(uint32((i << 5) + j)) {
					list = append(list, uint32((i<<5)+j))
				}
			}
		}
	}
	return list
}

// GetCompressed returns index list that has compressed.
func (s *BitArray) GetCompressed() ([]uint32, Type) {
	typ := WAHType
	s.changeTypeIfNeeded()
	if s.state == IndexesType {
		typ = IndexesType
		return s.getOffsets(), typ
	} else if len(s.uncompressed) == 0 {
		return s.uncompressed, typ
	}
	data := s.uncompressed
	s.compress(data)
	d := append([]uint32(nil), s.compressed...)
	return d, typ
}

// Set sets the bit at a specific position in the BitArray to the specified value.
func (s *BitArray) Set(index uint32, val bool) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.set(index, val)
}

// Get gets the value of the bit at a specific position in the BitArray.
func (s *BitArray) Get(index uint32) bool {
	s.mux.Lock()
	defer s.mux.Unlock()
	return s.get(index)
}

// Parse creates new BitArray with given data.
func Create(typ Type, ints []uint32) *BitArray {
	s := &BitArray{
		state:   typ,
		offsets: make(map[uint32]bool),
	}
	switch typ {
	case WAHType:
		s.compressed = ints
		s.uncompress()
		s.state = BitArrayType
		s.compressed = s.compressed[:0]
	case BitArrayType:
		s.uncompressed = ints
	case IndexesType:
		for _, i := range ints {
			s.offsets[i] = true
		}
	}
	return s
}

// New returns a new BitArray.
func New() *BitArray {
	return &BitArray{
		state:   IndexesType,
		offsets: make(map[uint32]bool),
	}
}
