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
)

type LocalMemory struct {
	data []byte
	size uint32
}

func (s *LocalMemory) PutUint32(offset uint32, value uint32) {
	binary.BigEndian.PutUint32(s.data[offset:], value)
}

func (s *LocalMemory) Uint32(offset uint32) uint32 {
	return binary.BigEndian.Uint32(s.data[offset:])
}

func (s *LocalMemory) Read(offset uint32, size uint32) ([]byte, error) {
	if offset+size > s.size {
		return nil, ErrInvalidOffset
	}

	return s.data[offset : offset+size], nil
}

func (s *LocalMemory) Write(offset uint32, src []byte) {
	copy(s.data[offset:], src)
}

func (s *LocalMemory) Size() uint32 {
	return s.size
}

func NewLocalMemory(size uint32) *LocalMemory {
	return &LocalMemory{
		data: make([]byte, size),
		size: size,
	}
}

type LocalReader struct {
	poll chan Header
}

func (r *LocalReader) SendHeader(header Header) {
	r.poll <- header
}

func (r *LocalReader) Poll() (Header, error) {
	header := <-r.poll
	return header, nil
}

func NewLocalReader(memory Memory) *LocalReader {
	return &LocalReader{
		poll: make(chan Header, 1000),
	}
}
