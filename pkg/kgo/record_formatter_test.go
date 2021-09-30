package kgo

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRecordFormatter(t *testing.T) {
	r := &Record{
		Key:   []byte("key"),
		Value: []byte("value"),
		Headers: []RecordHeader{
			{"H1", []byte("V1")},
			{"h2", []byte("v2")},
		},
		Timestamp:     time.Unix(17, 0),
		Topic:         "topictopictopictopictopict",
		Partition:     3,
		ProducerEpoch: 1,
		ProducerID:    791,
		LeaderEpoch:   -1,
		Offset:        343,
	}
	p := &FetchPartition{
		HighWatermark:    999,
		LastStableOffset: 666,
		LogStartOffset:   333,
	}

	for _, test := range []struct {
		layout string
		expR   string
		expP   string // defaults to expR if empty
	}{
		{
			layout: "%v",
			expR:   "value",
		},

		{
			layout: "%T{hex16}%t %V{ascii} %v %V{little16} %k %K{big32} %o",
			expR:   "001atopictopictopictopictopict 5 value \x05\x00 key \x00\x00\x00\x03 343",
		},

		{
			layout: "%[ %| %]",
			expR:   "<nil> <nil> <nil>",
			expP:   "333 666 999",
		},

		{
			layout: "%d{strftime## %a ##} %d %d{ascii}",
			expR:   " Wed  17000 17000",
		},

		{
			layout: "%T{ascii} %T{hex64} %T{hex32} %T{hex16} %T{hex8} %T{hex4} %T{hex}",
			expR:   "26 000000000000001a 0000001a 001a 1a a 1a",
		},

		{
			layout: "%K{big64} %K{big32} %K{big16} %K{big8}",
			expR:   "\x00\x00\x00\x00\x00\x00\x00\x03 \x00\x00\x00\x03 \x00\x03 \x03",
		},

		{
			layout: "%K{little64} %K{little32} %K{little16} %K{little8}",
			expR:   "\x03\x00\x00\x00\x00\x00\x00\x00 \x03\x00\x00\x00 \x03\x00 \x03",
		},

		{
			layout: `\t\r\n\\\x00 %{%}%%`,
			expR:   "\t\r\n\\\x00 {}%",
		},

		{
			layout: "%T %K %V %H %p %o %e %i %x %y",
			expR:   "26 3 5 2 3 343 -1 1 791 1",
			expP:   "26 3 5 2 3 343 -1 2 791 1",
		},

		{
			layout: "%k{base64} %k{hex}",
			expR:   "a2V5 6b6579",
		},

		{
			layout: "%H %h{ %K{ascii} %k %v %V } %k %v",
			expR:   "2  2 H1 V1 2  2 h2 v2 2  key value",
		},

		//
	} {
		f, err := NewRecordFormatter(test.layout)
		if err != nil {
			t.Errorf("%s: unexpected err: %v", test.layout, err)
			continue
		}

		gotR := string(f.AppendRecord(nil, r))
		gotP := string(f.AppendPartitionRecord(nil, p, r))

		if gotR != test.expR {
			t.Errorf("R[%s]: got %s != exp %s", test.layout, gotR, test.expR)
		}

		// Partition formatting defaults to the record format if the
		// expectation is empty.
		expP := test.expP
		if expP == "" {
			expP = test.expR
		}
		if gotP != expP {
			t.Errorf("P[%s]: got %s != exp %s", test.layout, gotP, expP)
		}
	}
}

func TestRecordReader(t *testing.T) {
	for _, test := range []struct {
		layout string
		in     string
		exp    []*Record
	}{
		{
			layout: "%v",
			in:     "foo bar biz\nbaz",
			exp:    []*Record{StringRecord("foo bar biz\nbaz")},
		},

		{
			layout: "%k %v",
			in:     "foo bar biz\nbaz",
			exp:    []*Record{KeyStringRecord("foo", "bar biz\nbaz")},
		},

		{
			layout: "%k %v\n",
			in:     "foo bar biz\nbaz \n biz\n",
			exp: []*Record{
				KeyStringRecord("foo", "bar biz"),
				KeyStringRecord("baz", ""),
				KeyStringRecord("", "biz"),
			},
		},

		{
			layout: "%t %k %v",
			in:     "foo bar biz",
			exp: []*Record{
				&Record{Topic: "foo", Key: []byte("bar"), Value: []byte("biz")},
			},
		},

		{
			layout: "%T%t %K%k %V{byte}%v",
			in:     "3foo 3bar \x03biz",
			exp: []*Record{
				&Record{Topic: "foo", Key: []byte("bar"), Value: []byte("biz")},
			},
		},

		{
			layout: "%T%to %k %v",
			in:     "3fooo bar biz",
			exp: []*Record{
				&Record{Topic: "foo", Key: []byte("bar"), Value: []byte("biz")},
			},
		},

		{
			layout: "%K{ascii}%k",
			in:     "3foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K%k",
			in:     "3foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{hex64}%k",
			in:     "0000000000000003foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{hex32}%k",
			in:     "00000003foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{hex16}%k",
			in:     "0003foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{hex8}%k",
			in:     "03foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{hex4}%k",
			in:     "3foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{big64}%k",
			in:     "\x00\x00\x00\x00\x00\x00\x00\x03foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{big32}%k",
			in:     "\x00\x00\x00\x03foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{big16}%k",
			in:     "\x00\x03foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{big8}%k",
			in:     "\x03foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{little64}%k",
			in:     "\x03\x00\x00\x00\x00\x00\x00\x00foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{little32}%k",
			in:     "\x03\x00\x00\x00foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{little16}%k",
			in:     "\x03\x00foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{little8}%k",
			in:     "\x03foo",
			exp:    []*Record{KeyStringRecord("foo", "")},
		},
		{
			layout: "%K{3}%kgap%V{3}%v",
			in:     "foogapbar",
			exp:    []*Record{KeyStringRecord("foo", "bar")},
		},

		{
			layout: `\t\r\n\\\x00 %{%}%% %v`,
			in:     "\t\r\n\\\x00 {}% foo",
			exp:    []*Record{StringRecord("foo")},
		},

		{
			layout: "%H{2}%V{ascii}%v%h{%V%v%K%k}",
			in:     "3foo1v1k2vv2kk",
			exp: []*Record{
				&Record{
					Value: []byte("foo"),
					Headers: []RecordHeader{
						{"k", []byte("v")},
						{"kk", []byte("vv")},
					},
				},
			},
		},

		{
			layout: "%V{3}%v bar",
			in:     "foo bar",
			exp:    []*Record{StringRecord("foo")},
		},

		//
	} {
		r, err := NewRecordReader(strings.NewReader(test.in), -1, test.layout)
		if err != nil {
			t.Errorf("%s: unexpected err: %v", test.layout, err)
			continue
		}
		for i, exp := range test.exp {
			rec, err := r.ReadRecord()
			if err != nil {
				t.Errorf("%d %s: unable to read record: %v", i, test.layout, err)
				continue
			}
			if !reflect.DeepEqual(rec, exp) {
				t.Errorf("%d %s:\ngot %#v\nexp %#v", i, test.layout, rec, exp)
			}
		}
	}
}
