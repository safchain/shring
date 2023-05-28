package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

var (
	ErrInvalidStartWatermark = errors.New("invalid start watermark")
	ErrInvalidEndWatermark   = errors.New("invalid end watermark")
	ErrInvalidOffset         = fmt.Errorf("invalid offset")
)

type Header struct {
	Watermark uint32
	Offset    uint32
}

type Block struct {
	Data []byte
	Err  error
}

type SHMem struct {
	data []byte
	size uint32
}

func (s *SHMem) PutUint32(offset uint32, value uint32) {
	binary.BigEndian.PutUint32(s.data[offset:], value)
}

func (s *SHMem) Uint32(offset uint32) uint32 {
	return binary.BigEndian.Uint32(s.data[offset:])
}

func (s *SHMem) Read(offset uint32, size uint32) ([]byte, error) {
	if offset+size > s.size {
		return nil, ErrInvalidOffset
	}

	return s.data[offset : offset+size], nil
}

func (s *SHMem) Write(offset uint32, src []byte) {
	copy(s.data[offset:], src)
}

type SHRing struct {
	shmem        *SHMem
	writerOffset uint32
	readers      []Reader
	watermark    uint32
	index        uint32
}

type Reader struct {
	shmem     *SHMem
	poll      chan Header
	lastIndex uint32
}

func NewSHRing(size uint32) *SHRing {
	return &SHRing{
		shmem: &SHMem{
			data: make([]byte, size),
			size: size,
		},
		watermark: uint32(rand.Intn(math.MaxUint32)),
	}
}

func (r *SHRing) NewReader() Reader {
	reader := Reader{
		poll:  make(chan Header, r.shmem.size),
		shmem: r.shmem,
	}
	r.readers = append(r.readers, reader)

	return reader
}

func (r *SHRing) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	size := 4*4 + len(data) // watermark + id + size + watermark + data

	// not enough space to write the current entry
	// and at least the watermark and the size of the next entry
	if uint32(size) > r.shmem.size-r.writerOffset {
		// restart at the beginning
		r.writerOffset = 0
	}
	offset := r.writerOffset

	// put watermark
	r.shmem.PutUint32(r.writerOffset, r.watermark)
	r.writerOffset += 4

	// put index
	r.shmem.PutUint32(r.writerOffset, r.index+1)
	r.writerOffset += 4

	// put size
	r.shmem.PutUint32(r.writerOffset, uint32(len(data)))
	r.writerOffset += 4

	// put data
	r.shmem.Write(r.writerOffset, data)
	r.writerOffset += uint32(len(data))

	// put watermark
	r.shmem.PutUint32(r.writerOffset, r.watermark)
	r.writerOffset += 4

	r.notifyReaders(r.watermark, offset)

	// increment watermark and index
	r.watermark++
	r.index++

	return len(data), nil
}

func (r *SHRing) notifyReaders(watermark uint32, offset uint32) {
	for _, reader := range r.readers {
		select {
		case reader.poll <- Header{Watermark: watermark, Offset: offset}:
		default:
		}
	}
}

func (r *Reader) readData(offset uint32, watermark uint32) ([]byte, uint32, error) {
	var lost uint32

	if r.shmem.Uint32(offset) != watermark {
		return nil, lost, ErrInvalidStartWatermark
	}
	offset += 4

	index := r.shmem.Uint32(offset)
	offset += 4

	if index != r.lastIndex+1 {
		if index > r.lastIndex {
			lost = index - r.lastIndex - 1
		} else {
			// workaround for uint32 overflow
			lost = 1
		}
	}

	size := r.shmem.Uint32(offset)
	if size > r.shmem.size-offset-4 {
		return nil, lost, ErrInvalidEndWatermark
	}
	offset += 4

	data, err := r.shmem.Read(offset, size)
	if err != nil {
		return nil, lost, err
	}
	offset += size

	if r.shmem.Uint32(offset) != watermark {
		return nil, lost, ErrInvalidEndWatermark
	}

	r.lastIndex = index

	return data, lost, nil
}

func (r *Reader) Read() (<-chan Block, <-chan uint32) {
	blockCh := make(chan Block, 1000)
	lostCh := make(chan uint32, 1000)

	go func() {
		var (
			data []byte
			lost uint32
			err  error
		)

	LOOP:
		for {
			header := <-r.poll
		RETRY:
			for i := 0; i < 10; i++ {
				data, lost, err = r.readData(header.Offset, header.Watermark)
				switch err {
				case nil:
					break RETRY
				case ErrInvalidEndWatermark:
					time.Sleep(time.Millisecond)
				default:
					continue LOOP
				}
			}

			select {
			case blockCh <- Block{Data: data, Err: err}:
			default:
			}

			if lost > 0 {
				select {
				case lostCh <- lost:
				default:
				}
			}
		}
	}()

	return blockCh, lostCh
}

func reader(ring *SHRing) {
	reader := ring.NewReader()

	block, lost := reader.Read()

	for {
		select {
		case l := <-lost:
			fmt.Printf("Reader: lost %d\n", l)
		case b := <-block:
			fmt.Printf("Reader: %s; %v\n", string(b.Data), b.Err)
		}
	}
}

func writer(ring *SHRing) {
	var i int

	for {
		data := fmt.Sprintf("hello %d", i)
		ring.Write([]byte(data))
		time.Sleep(time.Millisecond)

		i++
	}
}

func main() {
	r := NewSHRing(10000)

	go reader(r)
	go reader(r)
	go reader(r)
	writer(r)
}
