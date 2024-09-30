package controller

func toIntPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}
