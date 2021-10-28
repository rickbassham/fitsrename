package fits

import (
	"io"
	"log"

	"github.com/astrogo/fitsio"
	"github.com/rickbassham/fitsrename/common"
)

type Decoder struct {
	rdr io.Reader
}

func NewDecoder(rdr io.Reader) *Decoder {
	return &Decoder{rdr: rdr}
}

func (d *Decoder) ReadHeader() (h common.Header, err error) {
	fit, err := fitsio.Open(d.rdr)
	if err != nil {
		log.Fatalln(err.Error())
	}
	defer fit.Close()

	hdr := fit.HDU(0).Header()

	keys := hdr.Keys()

	h = common.Header{}

	for _, key := range keys {
		v := hdr.Get(key).Value

		h[key] = v
	}

	return h, nil
}
