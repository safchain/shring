/*
Copyright © 2023 SYLVAIN AFCHAIN

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package shring

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

type RemoteReader interface {
	SendHeader(Header)
}

type Header struct {
	Watermark uint32
	Offset    uint32
}

func (h *Header) MarshalBinary() []byte {
	var data [8]byte

	binary.BigEndian.PutUint32(data[0:], h.Watermark)
	binary.BigEndian.PutUint32(data[4:], h.Offset)

	return data[:]
}

func (h *Header) UnmarshalBinary(data [8]byte) error {
	h.Watermark = binary.BigEndian.Uint32(data[0:])
	h.Offset = binary.BigEndian.Uint32(data[4:])

	return nil
}

type Block struct {
	Data []byte
	Err  error
}

type RingBuffer struct {
	sync.RWMutex

	memory        Memory
	writerOffset  uint32
	remoteReaders []RemoteReader
	watermark     uint32
	index         uint32
}

type HeaderPoller interface {
	Poll() (Header, error)
}

type Reader struct {
	memory       Memory
	headerPoller HeaderPoller
	lastIndex    uint32
}

func NewReader(memory Memory, headerPoller HeaderPoller) *Reader {
	return &Reader{
		memory:       memory,
		headerPoller: headerPoller,
	}
}

func NewRingBuffer(memory Memory) *RingBuffer {
	return &RingBuffer{
		memory:    memory,
		watermark: uint32(rand.Intn(math.MaxUint32)),
	}
}

func (r *RingBuffer) AddRemoteReader(reader RemoteReader) {
	r.Lock()
	r.remoteReaders = append(r.remoteReaders, reader)
	r.Unlock()
}

func (r *RingBuffer) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	size := 4*4 + len(data) // watermark + id + size + watermark + data

	// not enough space to write the current entry
	// and at least the watermark and the size of the next entry
	if uint32(size) > r.memory.Size()-r.writerOffset {
		// restart at the beginning
		r.writerOffset = 0
	}
	offset := r.writerOffset

	// put watermark
	r.memory.PutUint32(r.writerOffset, r.watermark)
	r.writerOffset += 4

	// put index
	r.memory.PutUint32(r.writerOffset, r.index+1)
	r.writerOffset += 4

	// put size
	r.memory.PutUint32(r.writerOffset, uint32(len(data)))
	r.writerOffset += 4

	// put data
	r.memory.Write(r.writerOffset, data)
	r.writerOffset += uint32(len(data))

	// put watermark
	r.memory.PutUint32(r.writerOffset, r.watermark)
	r.writerOffset += 4

	r.notifyRemoteReaders(r.watermark, offset)

	// increment watermark and index
	r.watermark++
	r.index++

	return len(data), nil
}

func (r *RingBuffer) notifyRemoteReaders(watermark uint32, offset uint32) {
	r.RLock()
	for _, reader := range r.remoteReaders {
		reader.SendHeader(Header{Watermark: watermark, Offset: offset})
	}
	r.RUnlock()
}

func (r *Reader) readData(offset uint32, watermark uint32) ([]byte, uint32, error) {
	var lost uint32

	if r.memory.Uint32(offset) != watermark {
		return nil, lost, ErrInvalidStartWatermark
	}
	offset += 4

	index := r.memory.Uint32(offset)
	offset += 4

	if index != r.lastIndex+1 {
		if index > r.lastIndex {
			lost = index - r.lastIndex - 1
		} else {
			// workaround for uint32 overflow
			lost = 1
		}
	}

	size := r.memory.Uint32(offset)
	if size > r.memory.Size()-offset-4 {
		return nil, lost, ErrInvalidEndWatermark
	}
	offset += 4

	data, err := r.memory.Read(offset, size)
	if err != nil {
		return nil, lost, err
	}
	offset += size

	if r.memory.Uint32(offset) != watermark {
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
			header Header
			data   []byte
			lost   uint32
			err    error
		)

	LOOP:
		for {
			header, err = r.headerPoller.Poll()
			if err == nil {
			RETRY:
				for i := 0; i < 10; i++ {
					data, lost, err = r.readData(header.Offset, header.Watermark)
					switch err {
					case nil:
						break RETRY
					case ErrInvalidEndWatermark:
						time.Sleep(time.Millisecond)
					default:
						fmt.Printf("Error: %+v\n", err)
						continue LOOP
					}
				}
			}

			if err != nil {
				fmt.Printf("Error: %+v\n", err)
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
