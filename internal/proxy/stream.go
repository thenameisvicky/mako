package proxy

import (
	"io"
	"sync"
)

var BufferPool = sync.Pool{
	New: func() interface{} {
		// Return 32KB buffer slices directly
		return make([]byte, 32<<10)
	},
}

// streamCopy copies from src to dst using a pooled buffer, and updates the session byte counters.
func streamCopy(dst io.Writer, src io.Reader, upload bool) (int64, error) {
	bufVal := BufferPool.Get().([]byte)
	defer BufferPool.Put(bufVal)

	n, err := io.CopyBuffer(dst, src, bufVal)
	if n > 0 {
		if upload {
			AddBytesUploaded(uint64(n))
		} else {
			AddBytesDownloaded(uint64(n))
		}
	}
	return n, err
}
