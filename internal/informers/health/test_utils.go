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

package health

import (
	"sync"
)

var (
	factoriesMu sync.RWMutex
)

// registerHealthInformerFactory is a test utility function and should not be used in production code.
// It is helpful for replacing the health informer factory with a mock factory for testing.
func registerHealthInformerFactory(informerType HealthInformerType, factory healthInformerFactory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	healthInformerFactories[informerType] = factory
}
