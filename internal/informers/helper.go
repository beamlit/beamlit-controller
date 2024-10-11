/*
Copyright 2024.

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

package informers

import (
	"fmt"
)

// errWrapper is a wrapper around an error that adds the model name to the error message.
// It is used to wrap the error returned by informers.
type ErrWrapper struct {
	ModelName string
	Err       error
}

func (e ErrWrapper) Error() string {
	return fmt.Errorf("model %s: %w", e.ModelName, e.Err).Error()
}
