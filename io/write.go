package io

import "io"

// WriteAll write all data to writer,
func WriteAll(data []byte, writer io.Writer) (int, error) {
	length := len(data)
	pos := 0
	for {
		m, err := writer.Write(data[pos:length])
		pos += m
		if err != nil {
			return pos, err
		}
		if pos == length {
			break
		}
	}
	return length, nil
}
