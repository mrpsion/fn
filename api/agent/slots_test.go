package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type testSlot struct {
	id       uint64
	err      error
	isClosed bool
}

func (a *testSlot) exec(ctx context.Context, call *call) error {
	return nil
}

func (a *testSlot) Close() error {
	if a.isClosed {
		panic(fmt.Errorf("id=%d already closed %v", a.id, a))
	}
	a.isClosed = true
	return nil
}

func (a *testSlot) Error() error {
	return a.err
}

func NewTestSlot(id uint64) Slot {
	mySlot := &testSlot{
		id: id,
	}
	return mySlot
}

func TestSlotQueueBasic1(t *testing.T) {

	maxId := uint64(10)
	slotName := "test1"

	slots := make([]Slot, 0, maxId)
	tokens := make([]*slotToken, 0, maxId)

	obj := NewSlotQueue(slotName)

	outChan, cancel := obj.startDequeuer(context.Background())
	select {
	case z := <-outChan:
		t.Fatalf("Should not get anything from queue: %#v", z)
	case <-time.After(time.Duration(500) * time.Millisecond):
	}
	cancel()

	// create slots
	for id := uint64(0); id < maxId; id += 1 {
		slots = append(slots, NewTestSlot(id))
	}

	// queue a few slots here
	for id := uint64(0); id < maxId; id += 1 {
		tok := obj.queueSlot(slots[id])

		innerTok := tok.slot.(*testSlot)

		// check for slot id match
		if innerTok != slots[id] {
			t.Fatalf("queued testSlot does not match with slotToken.slot %#v vs %#v", innerTok, slots[id])
		}

		tokens = append(tokens, tok)
	}

	// Now according to LIFO semantics, we should get 9,8,7,6,5,4,3,2,1,0 if we dequeued right now.
	// but let's eject 9
	if !obj.ejectSlot(tokens[9]) {
		t.Fatalf("Cannot eject slotToken: %#v", tokens[9])
	}
	// let eject 0
	if !obj.ejectSlot(tokens[0]) {
		t.Fatalf("Cannot eject slotToken: %#v", tokens[0])
	}
	// let eject 5
	if !obj.ejectSlot(tokens[5]) {
		t.Fatalf("Cannot eject slotToken: %#v", tokens[5])
	}
	// try ejecting 5 again, it should fail
	if obj.ejectSlot(tokens[5]) {
		t.Fatalf("Shouldn't be able to eject slotToken: %#v", tokens[5])
	}

	outChan, cancel = obj.startDequeuer(context.Background())

	// now we should get 8
	select {
	case z := <-outChan:
		if z.id != 8 {
			t.Fatalf("Bad slotToken received: %#v", z)
		}

		if !z.acquireSlot() {
			t.Fatalf("Cannot acquire slotToken received: %#v", z)
		}

		// second acquire shoudl fail
		if z.acquireSlot() {
			t.Fatalf("Should not be able to acquire twice slotToken: %#v", z)
		}

		z.slot.Close()

	case <-time.After(time.Duration(1) * time.Second):
		t.Fatal("timeout in waiting slotToken")
	}

	// now we should get 7
	select {
	case z := <-outChan:
		if z.id != 7 {
			t.Fatalf("Bad slotToken received: %#v", z)
		}

		// eject it before we can consume
		if !obj.ejectSlot(tokens[7]) {
			t.Fatalf("Cannot eject slotToken: %#v", tokens[2])
		}

		// we shouldn't be able to consume an ejected slotToken
		if z.acquireSlot() {
			t.Fatalf("We should not be able to acquire slotToken received: %#v", z)
		}

	case <-time.After(time.Duration(1) * time.Second):
		t.Fatal("timeout in waiting slotToken")
	}

	cancel()

	// we should get nothing or 6
	select {
	case z, ok := <-outChan:
		if ok {
			if z.id != 6 {
				t.Fatalf("Should not get anything except for 6 from queue: %#v", z)
			}
			if !z.acquireSlot() {
				t.Fatalf("cannot acquire token: %#v", z)
			}
		}
	case <-time.After(time.Duration(500) * time.Millisecond):
	}
}

func TestSlotQueueBasic2(t *testing.T) {

	obj := NewSlotQueue("test2")

	if !obj.isIdle() {
		t.Fatalf("Should be idle")
	}

	outChan, cancel := obj.startDequeuer(context.Background())
	select {
	case z := <-outChan:
		t.Fatalf("Should not get anything from queue: %#v", z)
	case <-time.After(time.Duration(500) * time.Millisecond):
	}

	cancel()
}

func statsHelperSet(runC, startC, waitC, runL, startL, waitL uint64) slotQueueStats {
	return slotQueueStats{
		states:    [SlotQueueLast]uint64{runC, startC, waitC},
		latencies: [SlotQueueLast]uint64{runL, startL, waitL},
	}
}

func TestSlotNewContainerLogic1(t *testing.T) {

	var cur slotQueueStats
	var prev slotQueueStats

	cur = statsHelperSet(0, 0, 0, 0, 0, 0)
	prev = statsHelperSet(0, 0, 0, 0, 0, 0)
	// CASE I: There's no one waiting despite cur == prev
	if isNewContainerNeeded(&cur, &prev) {
		t.Fatalf("Should not need a new container cur: %#v prev: %#v", cur, prev)
	}

	// CASE II: There are starters >= waiters
	cur = statsHelperSet(0, 10, 1, 0, 0, 0)
	prev = statsHelperSet(0, 10, 1, 0, 0, 0)
	if isNewContainerNeeded(&cur, &prev) {
		t.Fatalf("Should not need a new container cur: %#v prev: %#v", cur, prev)
	}

	// CASE III: no executors
	cur = statsHelperSet(0, 0, 1, 0, 0, 0)
	prev = statsHelperSet(0, 0, 1, 0, 0, 0)
	if !isNewContainerNeeded(&cur, &prev) {
		t.Fatalf("Should need a new container cur: %#v prev: %#v", cur, prev)
	}

	// CASE IV: cur == prev same, progress has stalled, with waiters and
	// small num of executors
	cur = statsHelperSet(2, 0, 10, 0, 0, 0)
	prev = statsHelperSet(2, 0, 10, 0, 0, 0)
	if !isNewContainerNeeded(&cur, &prev) {
		t.Fatalf("Should need a new container cur: %#v prev: %#v", cur, prev)
	}

	// CASE V: cur != prev, runLat/executors*2 < waitLat
	// Let's make cur and prev unequal to prevent blocked progress detection
	cur = statsHelperSet(2, 0, 10, 12, 100, 13)
	prev = statsHelperSet(2, 0, 10, 12, 101, 13)
	if !isNewContainerNeeded(&cur, &prev) {
		t.Fatalf("Should need a new container cur: %#v prev: %#v", cur, prev)
	}

	// CASE VI: cur != prev, runLat < waitLat
	cur = statsHelperSet(1, 0, 10, 12, 100, 14)
	prev = statsHelperSet(1, 0, 10, 12, 101, 14)
	if !isNewContainerNeeded(&cur, &prev) {
		t.Fatalf("Should need a new container cur: %#v prev: %#v", cur, prev)
	}

	// CAST VII: cur != prev, startLat < waitLat
	cur = statsHelperSet(1, 0, 10, 2, 10, 20)
	prev = statsHelperSet(1, 0, 10, 1, 11, 20)
	if !isNewContainerNeeded(&cur, &prev) {
		t.Fatalf("Should need a new container cur: %#v prev: %#v", cur, prev)
	}

	// CAST VIII: cur != prev, fallback
	cur = statsHelperSet(1, 0, 10, 2, 10, 2)
	prev = statsHelperSet(1, 0, 10, 1, 11, 2)
	if isNewContainerNeeded(&cur, &prev) {
		t.Fatalf("Should not need a new container cur: %#v prev: %#v", cur, prev)
	}
}

func TestSlotQueueBasic3(t *testing.T) {

	slotName := "test3"

	obj := NewSlotQueue(slotName)
	_, cancel1 := obj.startDequeuer(context.Background())

	slot1 := NewTestSlot(1)
	slot2 := NewTestSlot(2)
	token1 := obj.queueSlot(slot1)
	obj.queueSlot(slot2)

	// now our slot must be ready in outChan, but let's cancel it
	// to cause a requeue. This should cause [1, 2] ordering to [2, 1]
	cancel1()

	outChan, cancel2 := obj.startDequeuer(context.Background())

	// we should get '2' since cancel1() reordered the queue
	select {
	case item, ok := <-outChan:
		if !ok {
			t.Fatalf("outChan should be open")
		}

		inner := item.slot.(*testSlot)
		outer := slot2.(*testSlot)

		if inner.id != outer.id {
			t.Fatalf("item should be 2")
		}
		if inner.isClosed {
			t.Fatalf("2 should not yet be closed")
		}

		if !item.acquireSlot() {
			t.Fatalf("2 acquire should not fail")
		}

		item.slot.Close()

	case <-time.After(time.Duration(1) * time.Second):
		t.Fatal("timeout in waiting slotToken")
	}

	// let's eject 1
	if !obj.ejectSlot(token1) {
		t.Fatalf("failed to eject 1")
	}
	if !slot1.(*testSlot).isClosed {
		t.Fatalf("1 should be closed")
	}

	// spin up bunch of go routines, where each should get a non-acquirable
	// token or timeout due the imminent obj.destroySlotQueue()
	var wg sync.WaitGroup
	goMax := 10
	wg.Add(goMax)
	for i := 0; i < goMax; i += 1 {
		go func(id int) {
			ch, cancl := obj.startDequeuer(context.Background())
			defer cancl()
			defer wg.Done()

			select {
			case z := <-ch:
				t.Fatalf("%v we shouldn't get anything from queue %#v", id, z)
			case <-time.After(time.Duration(500) * time.Millisecond):
			}
		}(i)
	}

	// let's cancel after destroy this time
	cancel2()

	wg.Wait()

	select {
	case z := <-outChan:
		t.Fatalf("Should not get anything from queue: %#v", z)
	case <-time.After(time.Duration(500) * time.Millisecond):
	}

	// both should be closed
	if !slot1.(*testSlot).isClosed {
		t.Fatalf("item1 should be closed")
	}
	if !slot2.(*testSlot).isClosed {
		t.Fatalf("item2 should be closed")
	}
}
