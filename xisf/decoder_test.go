package xisf_test

import (
	"os"
	"testing"

	"github.com/rickbassham/fitsrename/xisf"
)

func TestDecoder(t *testing.T) {
	f, err := os.Open("../test/masterLight_BIN-1_EXPOSURE-600.00s_FILTER-L_Mono.xisf")
	if err != nil {
		t.Fatal(err)
	}

	d := xisf.NewDecoder(f)

	hdr, err := d.ReadHeader()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%#v", hdr)

	t.FailNow()
}
