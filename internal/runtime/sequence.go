package runtime

type SequenceCursor struct {
	current int64
}

func NewSequenceCursor(next int64) SequenceCursor {
	return SequenceCursor{current: next - 1}
}

func (c *SequenceCursor) Next() int64 {
	c.current++
	return c.current
}

func (c *SequenceCursor) Reserve(count int) int64 {
	start := c.current + 1
	c.current += int64(count)
	return start
}

func (c SequenceCursor) Current() int64 {
	return c.current
}
