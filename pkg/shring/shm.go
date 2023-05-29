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
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type Shm struct {
	LocalMemory
	fd int
}

func (s *Shm) Close() error {
	return unix.Close(s.fd)
}

func CreateShm(path string, pages int, perm uint32) (*Shm, error) {
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, perm)
	if err != nil {
		return nil, err
	}

	f := os.NewFile(uintptr(fd), "ring")
	f.Truncate(int64(pages * os.Getpagesize()))

	bytes, err := unix.Mmap(fd, 0, pages*os.Getpagesize(), unix.PROT_WRITE|unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	return &Shm{
		LocalMemory: LocalMemory{
			data: bytes,
			size: uint32(pages * os.Getpagesize()),
		},
		fd: fd,
	}, nil
}

func OpenShm(path string, pages int, perm uint32) (*Shm, error) {
	fmt.Println("OpenShm", path, pages)

	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, perm)
	if err != nil {
		return nil, err
	}

	bytes, err := unix.Mmap(fd, 0, pages*os.Getpagesize(), unix.PROT_READ, unix.MAP_SHARED)
	if err != nil {

		return nil, err
	}

	return &Shm{
		LocalMemory: LocalMemory{
			data: bytes,
			size: uint32(pages * os.Getpagesize()),
		},
		fd: fd,
	}, nil
}
