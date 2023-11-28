package main

import (
"encoding/binary"
"io"
"fmt"
)

var BE = binary.BigEndian

func readU8(r io.Reader) (uint8, error) {
	var result uint8
	err := binary.Read(r, BE, &result);
	return result, err
}

func readU16(r io.Reader) (uint16, error) {
	var result uint16
	err := binary.Read(r, BE, &result);
	return result, err
}
func readU32(r io.Reader) (uint32, error) {
	var result uint32
	err := binary.Read(r, BE, &result);
	return result, err
}
func readString(r io.Reader) (string, error) {
	l, err := readU8(r)
	if err == nil && l == 0 {
		err = fmt.Errorf("invalid string length 0")
	}
	if err != nil { return "", err}
	buf := make([]byte, l)
	_, err = io.ReadFull(r, buf)
	return string(buf), err
}

func readLString(r io.Reader) (string, error) {
	l, err := readU32(r)
	if err == nil && l == 0 {
		err = fmt.Errorf("invalid string length 0")
	}
	if err != nil { return "", err}
	buf := make([]byte, l)
	_, err = io.ReadFull(r, buf)
	return string(buf), err
}

func writeLString(w io.Writer, s string) error {
  writeU32(w,uint32(len(s)))
  _, err := w.Write([]byte(s))
  return err
}

func writeU8(w io.Writer, i uint8) error {
  return binary.Write(w, BE, i);
}

func writeU32(w io.Writer, i uint32) error {
  return binary.Write(w, BE, i);
}
