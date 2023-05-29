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
	"net"
)

type Client struct {
	reader *Reader
	conn   net.Conn
}

func NewClient(shmPath string, perm uint32, uds string, pages int) (*Client, error) {
	conn, err := net.Dial("unix", uds)
	if err != nil {

		return nil, err
	}

	shm, err := OpenShm(shmPath, pages, perm)
	if err != nil {
		return nil, err
	}

	client := &Client{
		conn: conn,
	}

	client.reader = NewReader(shm, client)

	return client, nil
}

func (c *Client) Read() (<-chan Block, <-chan uint32) {
	return c.reader.Read()
}

func (c *Client) Poll() (Header, error) {
	var (
		data   [8]byte
		header Header
	)

	n, err := c.conn.Read(data[:])
	if err != nil {
		return Header{}, err
	}

	if n != 8 {
		return Header{}, ErrInvalidHeader
	}

	if err != header.UnmarshalBinary(data) {
		return Header{}, err
	}

	return header, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
