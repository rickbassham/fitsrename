package xisf

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strconv"

	"github.com/rickbassham/fitsrename/common"
)

var stringRegex regexp.Regexp = *regexp.MustCompile(`^\'.*?\'$`)
var integerRegex regexp.Regexp = *regexp.MustCompile(`^[\+\-]?\d+$`)

type FITSKeyword struct {
	XMLName xml.Name `xml:"FITSKeyword"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:"value,attr"`
	Comment string   `xml:"comment,attr"`
}

type Image struct {
	XMLName      xml.Name      `xml:"Image"`
	FITSKeywords []FITSKeyword `xml:"FITSKeyword"`
}

type Xisf struct {
	XMLName xml.Name `xml:"xisf"`
	Image   Image    `xml:"Image"`
}

type Decoder struct {
	rdr io.Reader
}

func NewDecoder(rdr io.Reader) *Decoder {
	return &Decoder{rdr: rdr}
}

func (d *Decoder) checkSignature() error {
	signature := make([]byte, 8)
	n, err := d.rdr.Read(signature)

	if err != nil {
		return err
	}

	if n != 8 {
		return errors.New("invalid signature length")
	}

	if !bytes.Equal([]byte("XISF0100"), signature) {
		return errors.New("invalid signature content")
	}

	return nil
}

func (d *Decoder) getHeaderLength() (uint32, error) {
	headerLengthBytes := make([]byte, 4)
	n, err := d.rdr.Read(headerLengthBytes)

	if err != nil {
		return 0, err
	}

	if n != 4 {
		return 0, errors.New("invalid header length")
	}

	return binary.LittleEndian.Uint32(headerLengthBytes), nil
}

func (d *Decoder) skipReserved(count uint32) error {
	skip := make([]byte, count)

	n, err := d.rdr.Read(skip)

	if err != nil {
		return err
	}

	if n != int(count) {
		return errors.New("invalid header length")
	}

	return nil
}

func (d *Decoder) ReadHeader() (h common.Header, err error) {
	err = d.checkSignature()
	if err != nil {
		return h, err
	}

	headerLen, err := d.getHeaderLength()
	if err != nil {
		return h, err
	}

	err = d.skipReserved(4)
	if err != nil {
		return h, err
	}

	rawHeader := make([]byte, headerLen)

	n, err := d.rdr.Read(rawHeader)
	if err != nil {
		return h, err
	}

	if n != int(headerLen) {
		return h, errors.New("invalid header length")
	}

	img := Xisf{}

	err = xml.Unmarshal(rawHeader, &img)
	if err != nil {
		return h, err
	}

	h = common.Header{}

	for _, kw := range img.Image.FITSKeywords {
		if len(kw.Value) == 0 {
			h[kw.Name] = nil
			continue
		}

		if stringRegex.MatchString(kw.Value) {
			h[kw.Name] = kw.Value[1 : len(kw.Value)-1]
		} else if integerRegex.MatchString(kw.Value) {
			h[kw.Name], _ = strconv.ParseInt(kw.Value, 10, 64)
		} else if kw.Value == "T" {
			h[kw.Name] = true
		} else if kw.Value == "F" {
			h[kw.Name] = false
		} else {
			val, err := strconv.ParseFloat(kw.Value, 64)
			if err != nil {
				println(err.Error())
				continue
			}

			h[kw.Name] = val
		}
	}

	return h, nil
}
