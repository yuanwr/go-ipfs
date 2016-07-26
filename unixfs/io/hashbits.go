package io

type hashBits struct {
	b        []byte
	consumed int
}

func (hb *hashBits) Next(i int) uint {
	if i%8 != 0 {
		panic("cant currently do uneven bitsizes")
	}

	frag := hb.b[hb.consumed/8 : (hb.consumed+i)/8]
	hb.consumed += i

	return toInt(frag)
}

func toInt(b []byte) uint {
	var s uint
	for i, v := range b {
		s += uint(v << uint(i*8))
	}
	return s
}
