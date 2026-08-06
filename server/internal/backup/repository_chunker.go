package backup

import (
	"context"
	"fmt"
	"io"
)

const (
	repositoryChunkMin = 512 << 10
	repositoryChunkAvg = 1 << 20
	repositoryChunkMax = 4 << 20
)

// contentDefinedChunker implements the normalized FastCDC cut-point strategy.
// The rolling Gear hash only retains the latest 64 bytes through uint64
// overflow, so chunk boundaries re-synchronize after insertions or deletions.
type contentDefinedChunker struct {
	minSize   int
	avgSize   int
	maxSize   int
	smallMask uint64
	largeMask uint64
	gear      [256]uint64
}

func newContentDefinedChunker() *contentDefinedChunker {
	chunker := &contentDefinedChunker{
		minSize:   repositoryChunkMin,
		avgSize:   repositoryChunkAvg,
		maxSize:   repositoryChunkMax,
		smallMask: (1 << 21) - 1,
		largeMask: (1 << 19) - 1,
	}

	// SplitMix64 produces a stable, well-distributed Gear table. The seed and
	// generation algorithm are part of repository format v1 and must not change.
	seed := uint64(0x6a09e667f3bcc909)
	for i := range chunker.gear {
		seed += 0x9e3779b97f4a7c15
		value := seed
		value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
		value = (value ^ (value >> 27)) * 0x94d049bb133111eb
		chunker.gear[i] = value ^ (value >> 31)
	}
	return chunker
}

func (c *contentDefinedChunker) Split(ctx context.Context, reader io.Reader, emit func([]byte) error) error {
	if reader == nil || emit == nil {
		return fmt.Errorf("chunk reader and emitter are required")
	}

	pending := make([]byte, 0, c.maxSize+(256<<10))
	readBuffer := make([]byte, 256<<10)
	eof := false
	for {
		if !eof {
			readCount, readErr := reader.Read(readBuffer)
			if readCount > 0 {
				pending = append(pending, readBuffer[:readCount]...)
			}
			switch readErr {
			case nil:
			case io.EOF:
				eof = true
			default:
				return fmt.Errorf("read source for chunking: %w", readErr)
			}
			if readCount == 0 && readErr == nil {
				continue
			}
		}

		for len(pending) > 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			cut := c.findCutPoint(pending, eof)
			if cut == 0 {
				break
			}
			chunk := make([]byte, cut)
			copy(chunk, pending[:cut])
			if err := emit(chunk); err != nil {
				return err
			}
			copy(pending, pending[cut:])
			pending = pending[:len(pending)-cut]
		}

		if eof {
			if len(pending) != 0 {
				return fmt.Errorf("chunker stopped with %d buffered bytes", len(pending))
			}
			return nil
		}
	}
}

func (c *contentDefinedChunker) findCutPoint(data []byte, eof bool) int {
	if len(data) < c.minSize {
		if eof {
			return len(data)
		}
		return 0
	}

	limit := len(data)
	if limit > c.maxSize {
		limit = c.maxSize
	}
	var hash uint64
	for index := c.minSize; index < limit; index++ {
		hash = (hash << 1) + c.gear[data[index]]
		mask := c.largeMask
		if index < c.avgSize {
			mask = c.smallMask
		}
		if hash&mask == 0 {
			return index + 1
		}
	}
	if len(data) >= c.maxSize {
		return c.maxSize
	}
	if eof {
		return len(data)
	}
	return 0
}
