//go:build kafka

package kafka

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	ckf "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type shutdownConsumer struct {
	readStarted chan struct{}
	releaseRead chan struct{}
	message     *ckf.Message
	reading     atomic.Bool
	unsafeClose atomic.Bool
	closeCalls  atomic.Int32
	storeCalls  atomic.Int32
}

func (c *shutdownConsumer) ReadMessage(time.Duration) (*ckf.Message, error) {
	c.reading.Store(true)
	defer c.reading.Store(false)
	if c.readStarted != nil {
		close(c.readStarted)
		<-c.releaseRead
	}
	if c.message != nil {
		return c.message, nil
	}
	return nil, ckf.NewError(ckf.ErrTimedOut, "poll timed out", false)
}

func (c *shutdownConsumer) StoreMessage(*ckf.Message) ([]ckf.TopicPartition, error) {
	c.storeCalls.Add(1)
	return nil, nil
}

func (*shutdownConsumer) SubscribeTopics([]string, ckf.RebalanceCb) error { return nil }

func (c *shutdownConsumer) Close() error {
	c.closeCalls.Add(1)
	c.unsafeClose.Store(c.reading.Load())
	return nil
}

func awaitShutdown(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("client operation did not finish")
	}
}

func TestConsumerCloseWaitsForPolling(t *testing.T) {
	c := &shutdownConsumer{readStarted: make(chan struct{}), releaseRead: make(chan struct{})}
	cons := &Consumer{c: c}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		_, _ = cons.readMessage()
	}()
	awaitShutdown(t, c.readStarted)
	if cons.mu.TryLock() {
		cons.mu.Unlock()
		close(c.releaseRead)
		t.Fatal("native polling did not hold the lifecycle lock")
	}
	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		cons.Close()
	}()
	close(c.releaseRead)
	awaitShutdown(t, readDone)
	awaitShutdown(t, closeDone)
	cons.Close()
	if c.unsafeClose.Load() || c.closeCalls.Load() != 1 {
		t.Fatalf("unsafe close = %v, close calls = %d", c.unsafeClose.Load(), c.closeCalls.Load())
	}
	if _, err := cons.readMessage(); !errors.Is(err, ErrClientClosed) {
		t.Fatalf("poll after close: %v", err)
	}
	if err := cons.SubscribeTopics([]string{"jobs"}); !errors.Is(err, ErrClientClosed) {
		t.Fatalf("subscribe after close: %v", err)
	}
}

func TestConsumerCloseDuringHandlerDoesNotStoreAfterClose(t *testing.T) {
	c := &shutdownConsumer{message: &ckf.Message{}}
	cons := &Consumer{c: c}
	var consumeErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		consumeErr = cons.Consume(context.Background(), func(*ckf.Message) error {
			cons.Close()
			return nil
		})
	}()
	awaitShutdown(t, done)
	if !errors.Is(consumeErr, ErrClientClosed) || c.storeCalls.Load() != 0 {
		t.Fatalf("consume error = %v, stored offsets = %d", consumeErr, c.storeCalls.Load())
	}
}

func TestConsumerStoresCompletedMessageAfterContextCancellation(t *testing.T) {
	c := &shutdownConsumer{message: &ckf.Message{}}
	cons := &Consumer{c: c}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := cons.Consume(ctx, func(*ckf.Message) error {
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) || c.storeCalls.Load() != 1 {
		t.Fatalf("consume error = %v, stored offsets = %d", err, c.storeCalls.Load())
	}
}

type shutdownProducer struct {
	produceStarted chan struct{}
	releaseProduce chan struct{}
	flushStarted   chan struct{}
	releaseFlush   chan struct{}
	producing      atomic.Bool
	unsafeClose    atomic.Bool
	produceCalls   atomic.Int32
	flushCalls     atomic.Int32
	closeCalls     atomic.Int32
}

func (p *shutdownProducer) Produce(*ckf.Message, chan ckf.Event) error {
	p.produceCalls.Add(1)
	p.producing.Store(true)
	defer p.producing.Store(false)
	close(p.produceStarted)
	<-p.releaseProduce
	return nil
}

func (p *shutdownProducer) Flush(int) int {
	p.flushCalls.Add(1)
	p.unsafeClose.Store(p.producing.Load())
	if p.flushStarted != nil {
		close(p.flushStarted)
		<-p.releaseFlush
	}
	return 0
}

func TestProducerRejectsPublicationWhileFlushIsPending(t *testing.T) {
	p := &shutdownProducer{flushStarted: make(chan struct{}), releaseFlush: make(chan struct{})}
	pr := &Producer{p: p}
	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		pr.Close()
	}()
	awaitShutdown(t, p.flushStarted)
	produceDone := make(chan struct{})
	go func() {
		defer close(produceDone)
		if err := pr.Produce("jobs", nil, nil); !errors.Is(err, ErrClientClosed) {
			t.Errorf("produce during flush: %v", err)
		}
	}()
	t.Cleanup(func() {
		close(p.releaseFlush)
		awaitShutdown(t, closeDone)
	})
	awaitShutdown(t, produceDone)
	if p.produceCalls.Load() != 0 {
		t.Fatal("native publication was attempted during flush")
	}
}

func (p *shutdownProducer) Close() { p.closeCalls.Add(1) }

func TestProducerCloseWaitsForPublication(t *testing.T) {
	p := &shutdownProducer{produceStarted: make(chan struct{}), releaseProduce: make(chan struct{})}
	pr := &Producer{p: p}
	produceDone := make(chan struct{})
	go func() {
		defer close(produceDone)
		if err := pr.Produce("jobs", nil, nil); err != nil {
			t.Errorf("produce: %v", err)
		}
	}()
	awaitShutdown(t, p.produceStarted)
	if pr.mu.TryLock() {
		pr.mu.Unlock()
		close(p.releaseProduce)
		t.Fatal("native publication did not hold the lifecycle lock")
	}
	closeDone := make(chan struct{})
	go func() {
		defer close(closeDone)
		pr.Close()
	}()
	close(p.releaseProduce)
	awaitShutdown(t, produceDone)
	awaitShutdown(t, closeDone)
	pr.Close()
	if err := pr.Produce("jobs", nil, nil); !errors.Is(err, ErrClientClosed) {
		t.Fatalf("produce after close: %v", err)
	}
	if p.unsafeClose.Load() || p.flushCalls.Load() != 1 || p.closeCalls.Load() != 1 || p.produceCalls.Load() != 1 {
		t.Fatalf("unsafe close = %v, flush/close/produce calls = %d/%d/%d", p.unsafeClose.Load(), p.flushCalls.Load(), p.closeCalls.Load(), p.produceCalls.Load())
	}
}
