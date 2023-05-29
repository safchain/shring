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
	"context"
	"net"
	"os"
	"sync"
)

type Server struct {
	sync.RWMutex

	ring     *RingBuffer
	clients  []*RemoteClient
	listener net.Listener
}

type RemoteClient struct {
	conn         net.Conn
	onWriteError func(_ *RemoteClient, _ error)
}

func (r *RemoteClient) SendHeader(header Header) {
	_, err := r.conn.Write(header.MarshalBinary()[:])
	if err != nil {
		r.onWriteError(r, err)
	}
}

func NewServer(shmPath string, perm uint32, uds string, pages int) (*Server, error) {
	os.RemoveAll(shmPath)
	os.RemoveAll(uds)

	listener, err := net.Listen("unix", uds)
	if err != nil {
		return nil, err
	}

	shm, err := CreateShm(shmPath, pages, perm)
	if err != nil {
		return nil, err
	}

	return &Server{
		listener: listener,
		ring:     NewRingBuffer(shm),
	}, nil
}

func (s *Server) accept(ctx context.Context, errCh chan error) {
	onWriteError := func(client *RemoteClient, err error) {
		s.Lock()
		for i, c := range s.clients {
			if c == client {
				s.clients = append(s.clients[:i], s.clients[i+1:]...)
				break
			}
		}
		s.Unlock()
	}

LOOP:
	for {
		select {
		case <-ctx.Done():
			break LOOP
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				continue
			}

			client := &RemoteClient{conn: conn}
			client.onWriteError = onWriteError

			s.Lock()
			s.clients = append(s.clients, client)
			s.Unlock()

			s.ring.AddRemoteReader(client)
		}
	}

	s.listener.Close()
}

func (s *Server) Write(data []byte) {
	s.ring.Write(data)
}

func (s *Server) Start(ctx context.Context) <-chan error {
	errCh := make(chan error)
	go s.accept(ctx, errCh)
	return errCh
}
