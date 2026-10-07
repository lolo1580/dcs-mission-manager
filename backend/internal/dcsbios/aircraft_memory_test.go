package dcsbios

import "testing"

func TestAircraftChangeDiscardsPreviousExports(t *testing.T) {
	c := New(DefaultOptions(), nil)
	c.applyFrame(Frame{Blocks: []Block{{Address: AcftNameAddress, Data: append([]byte("OldJet"), 0)}, {Address: 100, Data: []byte{42, 0}}}})
	c.applyFrame(Frame{Blocks: []Block{{Address: AcftNameAddress, Data: append([]byte("NewJet"), 0)}, {Address: 200, Data: []byte{7, 0}}}})
	mem := c.Memory()
	if _, ok := mem[100]; ok {
		t.Fatal("previous aircraft's value retained")
	}
	if mem[200] != 7 {
		t.Fatal("new frame's export was discarded")
	}
}
