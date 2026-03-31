package service

import (
	"io"
	"sync/atomic"
	"time"
)

// progressCallback 在每次读取时被调用，报告已读字节数和估算速率。
type progressCallback func(bytesRead int64, speedBps float64)

// progressReader 包装 io.Reader，定期通过回调报告传输进度。
type progressReader struct {
	reader    io.Reader
	total     int64
	read      atomic.Int64
	callback  progressCallback
	startTime time.Time
	lastCall  time.Time
	interval  time.Duration
}

func newProgressReader(reader io.Reader, total int64, callback progressCallback) *progressReader {
	now := time.Now()
	return &progressReader{
		reader:    reader,
		total:     total,
		callback:  callback,
		startTime: now,
		lastCall:  now,
		interval:  500 * time.Millisecond,
	}
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		current := r.read.Add(int64(n))
		now := time.Now()
		isFinal := err == io.EOF || (r.total > 0 && current >= r.total)
		if isFinal || now.Sub(r.lastCall) >= r.interval {
			r.lastCall = now
			elapsed := now.Sub(r.startTime).Seconds()
			speed := float64(0)
			if elapsed > 0 {
				speed = float64(current) / elapsed
			}
			r.callback(current, speed)
		}
	}
	return n, err
}
