package chess

type SquareSet struct {
	mask uint64
}

func NewSquareSet(mask uint64) *SquareSet {
	return &SquareSet{mask}
}

func (s *SquareSet) Iter() <-chan int {
	ch := make(chan int)
	go func() {
        square := bitScan(s.mask, 0)
        for square != -1 {
            ch <- square
            square = bitScan(s.mask, square+1)
        }
	}()
	return ch
}
