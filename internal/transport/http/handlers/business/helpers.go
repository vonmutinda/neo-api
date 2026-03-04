package business

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
