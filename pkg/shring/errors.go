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
	"errors"
	"fmt"
)

var (
	ErrInvalidStartWatermark = errors.New("invalid start watermark")
	ErrInvalidEndWatermark   = errors.New("invalid end watermark")
	ErrInvalidOffset         = fmt.Errorf("invalid offset")
	ErrInvalidHeader         = fmt.Errorf("invalid header")
)
