package rule

type sizeChecker struct {
	maxSize int
}

func (rbs *sizeChecker) MaxSize() int {
	return rbs.maxSize
}
