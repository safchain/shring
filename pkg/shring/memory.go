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

type Memory interface {
	Size() uint32
	PutUint32(offset uint32, value uint32)
	Uint32(offset uint32) uint32
	Read(offset uint32, size uint32) ([]byte, error)
	Write(offset uint32, src []byte)
}
