package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
)

var (
	ErrInvalidStartWatermark = errors.New("invalid start watermark")
	ErrInvalidEndWatermark   = errors.New("invalid end watermark")
	ErrInvalidOffset         = fmt.Errorf("invalid offset")
)

type Block struct {
	Data []byte
	Err  error
}

type SHMem struct {
	data []byte
	size uint64
}

func (s *SHMem) PutUint32(offset uint64, value uint32) {
	binary.BigEndian.PutUint32(s.data[offset:], value)
}

func (s *SHMem) Uint32(offset uint64) uint32 {
	return binary.BigEndian.Uint32(s.data[offset:])
}

func (s *SHMem) Read(offset uint64, size uint64) ([]byte, error) {
	if offset+size > s.size {
		return nil, ErrInvalidOffset
	}

	return s.data[offset : offset+size], nil
}

func (s *SHMem) Write(offset uint64, src []byte) {
	copy(s.data[offset:], src)
}

type SHRing struct {
	shmem        *SHMem
	writerOffset uint64
	readers      []Reader
	watermark    uint32
}

type Reader struct {
	shmem     *SHMem
	watermark uint32
	poll      chan uint64
}

func NewSHRing(size int) *SHRing {
	return &SHRing{
		shmem: &SHMem{
			data: make([]byte, size),
			size: uint64(size),
		},
		watermark: uint32(rand.Intn(math.MaxUint32)),
	}
}

func (r *SHRing) NewReader() Reader {
	reader := Reader{
		poll:      make(chan uint64),
		shmem:     r.shmem,
		watermark: r.watermark,
	}
	r.readers = append(r.readers, reader)

	return reader
}

func (r *SHRing) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	size := 4*3 + len(data)

	// not enough space to write the current entry
	// and at least the watermark and the size of the next entry
	if uint64(size) > r.shmem.size-r.writerOffset {
		// restart at the beginning
		r.writerOffset = 0
	}
	offset := r.writerOffset

	// put watermark
	r.shmem.PutUint32(r.writerOffset, r.watermark)
	r.writerOffset += 4

	// put size
	r.shmem.PutUint32(r.writerOffset, uint32(len(data)))
	r.writerOffset += 4

	// put data
	r.shmem.Write(r.writerOffset, data)
	r.writerOffset += uint64(len(data))

	// put watermark
	r.shmem.PutUint32(r.writerOffset, r.watermark)
	r.writerOffset += 4

	r.notifyReaders(offset)

	return len(data), nil
}

func (r *SHRing) notifyReaders(offset uint64) {
	for _, reader := range r.readers {
		select {
		case reader.poll <- offset:
		default:
		}
	}
}

func (r *Reader) readData(offset uint64) ([]byte, error) {
	watermark := r.shmem.Uint32(offset)
	if watermark != r.watermark {
		return nil, ErrInvalidStartWatermark
	}
	offset += 4

	size := r.shmem.Uint32(offset)
	offset += 4

	data, err := r.shmem.Read(offset, uint64(size))
	if err != nil {
		return nil, err
	}
	offset += uint64(size)

	watermark = r.shmem.Uint32(offset)
	if watermark != r.watermark {
		return nil, ErrInvalidEndWatermark
	}

	return data, nil
}

func (r *Reader) Read() <-chan Block {
	ch := make(chan Block)

	go func() {
		for {
			offset := <-r.poll
			data, err := r.readData(offset)
			// TODO create another chan the generate lost events
			select {
			case ch <- Block{Data: data, Err: err}:
			default:
			}
		}
	}()

	return ch
}

func reader(ring *SHRing) {
	reader := ring.NewReader()

	for block := range reader.Read() {
		fmt.Printf("Reader: %s; %v\n", string(block.Data), block.Err)
	}
}

func writer(ring *SHRing) {
	var i int

	for {
		data := fmt.Sprintf("hello %d", i)
		ring.Write([]byte(data))
		//time.Sleep(time.Second)

		i++
	}
}

func main() {
	r := NewSHRing(100)

	go reader(r)
	go reader(r)
	go reader(r)
	writer(r)
}
