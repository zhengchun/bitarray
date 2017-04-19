bitarray 
===
Word Aligned Hybrid (WAH) Compression for BitArrays. 

A Golang implementation of the C# [WAHBitarray](https://www.codeproject.com/Articles/214997/Word-Aligned-Hybrid-WAH-Compression-for-BitArrays).

Install
===
> github.com/zhengchun/bitarray

Tutorial
===
```go
package main
func main() {
    b1 := bitarray.New()
    for i := 0; i < 11; i++ {
        b1.Set(uint32(i), true)
    }
    fmt.Println(b1.Get(uint32(10)))  // true
    fmt.Println(b1.Get(uint32(100))) // false

    bits, typ := b1.GetCompressed()
    b2 := bitarray.Create(typ, bits)
    fmt.Println(b2.GetBitIndexes()) // output: 0,1,2,....,10
    b2.Set(uint32(1), false)

    x := b1.Xor(b2)
    fmt.Println(x.GetBitIndexes()) // output: 1
}
```