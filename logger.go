/*

	 NOTE:
	ok so we want something that will switch between writing to an sse
	stream, and just writing to stdout, dynamically, so, do we want this
	thing to do that, or do we want to do that at a higher level, and just
	make this thing that writes to an sse stream

*/

package main

type LogWriter struct {
	sseChan chan<- []byte
}

func (l *LogWriter) Write(p []byte) (n int, err error) {
	l.sseChan <- p
	return len(p), nil
}
