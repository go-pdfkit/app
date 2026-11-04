package main

import (
	"encoding/json"
	"strconv"
	"sync"
	"testing"
)

// newTestConn is a conn with no websocket: everything the reply table does is
// reachable without a browser, which is the point of having pulled it out of
// pump and call.
func newTestConn() *conn {
	return &conn{replies: map[int]chan json.RawMessage{}, events: make(chan event, 4)}
}

// TestTheReplyTableSurvivesBothGoroutinesAtOnce is the test the defect needed
// and did not have.
//
// ⛔ RUN IT WITH -race, WHICH THE CI LANE DOES. pump reads and deletes from the
// reply table in its own goroutine while call inserts and deletes from the
// caller's. Before this, pump took no lock at all — under a comment saying the
// lock existed because "the process does not race quietly, it dies: concurrent
// map writes, mid-check, with the browser still open".
//
// A sequential check could never see it: check.go sends one command and waits
// for its reply, so the two goroutines touch the map at the same moment only
// when a reply lands while another call is registering. That window is why the
// defect sat there, and it is why this test opens it on purpose.
func TestTheReplyTableSurvivesBothGoroutinesAtOnce(t *testing.T) {
	c := newTestConn()
	const callers, each = 8, 200

	var wg sync.WaitGroup
	// The callers: reserve a number, wait for its reply, give the slot back.
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range each {
				id, ch := c.await()
				go c.deliver(id, json.RawMessage(strconv.Itoa(id)))
				<-ch
				c.forget(id)
			}
		}()
	}
	// And a goroutine standing in for pump answering numbers nobody asked
	// for, which is what a reply arriving after its call gave up looks like.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range callers * each {
			c.deliver(-i-1, json.RawMessage(`null`))
		}
	}()
	wg.Wait()

	// ⛔ And the table is EMPTY at the end. A race detector says nothing about
	// a leak: a conn that answered every call and kept every slot would pass
	// -race and grow without bound over a long check.
	c.mu.Lock()
	left := len(c.replies)
	c.mu.Unlock()
	if left != 0 {
		t.Errorf("%d reply slots were left behind by %d calls", left, callers*each)
	}
}

// TestAReplyNobodyIsWaitingForIsDropped: a reply whose call has given up — a
// timeout, a cancelled context — must not block the pump, which is the one
// goroutine reading the socket.
func TestAReplyNobodyIsWaitingForIsDropped(t *testing.T) {
	c := newTestConn()
	if c.deliver(1234, json.RawMessage(`{}`)) {
		t.Error("a reply to a request nobody made was reported as delivered")
	}

	id, ch := c.await()
	c.forget(id) // the caller gave up
	if c.deliver(id, json.RawMessage(`{}`)) {
		t.Error("a reply was delivered to a caller that had gone")
	}
	select {
	case got := <-ch:
		t.Errorf("the abandoned channel received %s", got)
	default:
	}
}

// TestEveryRequestGetsItsOwnNumber: the counter is handed out under the same
// lock as the table, so two callers cannot be given the same slot — which
// would silently hand one caller the other's reply.
func TestEveryRequestGetsItsOwnNumber(t *testing.T) {
	c := newTestConn()
	const n = 500
	ids := make(chan int, n)
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, _ := c.await()
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)

	seen := map[int]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatalf("request number %d was handed out twice: one caller would be given "+
				"the other's reply", id)
		}
		seen[id] = true
	}
	if len(seen) != n {
		t.Errorf("%d requests were given %d numbers", n, len(seen))
	}
}
