package config

func toPointer[T any](v T) *T {
	return &v
}
