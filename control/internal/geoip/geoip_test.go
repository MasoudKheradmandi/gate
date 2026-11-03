package geoip

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
)

const sample = `1.0.0.0,1.0.0.255,AU
1.0.1.0,1.0.3.255,CN
1.0.4.0,1.0.7.255,AU
8.8.8.0,8.8.8.255,US
"9.0.0.0","9.0.0.255","ZZ"
2001:200::,2001:200:ffff:ffff:ffff:ffff:ffff:ffff,JP
2001:4860::,2001:4860:ffff:ffff:ffff:ffff:ffff:ffff,US
`

func TestParseLookupEncode(t *testing.T) {
	db, err := ParseCSV(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	for ip, want := range map[string]string{
		"0.0.0.1": "XX", "1.0.0.7": "AU", "1.0.2.9": "CN", "1.0.5.1": "AU", "1.0.8.0": "XX",
		"8.8.8.8": "US", "8.8.9.0": "XX", "9.0.0.1": "XX", "255.255.255.255": "XX",
		"2001:200::1": "JP", "2001:4860:4860::8888": "US", "2001:4861::": "XX", "::1": "XX",
	} {
		if got := db.Lookup(net.ParseIP(ip)); got != want {
			t.Errorf("%s: got %s want %s", ip, got, want)
		}
	}
	var buf bytes.Buffer
	if err := db.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if string(b[:8]) != magic {
		t.Fatal("magic")
	}
	n4, n6 := binary.LittleEndian.Uint32(b[8:]), binary.LittleEndian.Uint32(b[12:])
	if int(n4) != len(db.v4) || int(n6) != len(db.v6) || len(b) != 16+int(n4)*6+int(n6)*18 {
		t.Fatalf("layout n4=%d n6=%d len=%d", n4, n6, len(b))
	}
}
