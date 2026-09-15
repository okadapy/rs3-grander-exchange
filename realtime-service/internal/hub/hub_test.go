package hub

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"
)

func testClient() *Client {
	return &Client{Send: make(chan []byte, 8), UserID: 1, Username: "trader"}
}

func drain(c *Client) []Envelope {
	var out []Envelope
	for {
		select {
		case msg := <-c.Send:
			var env Envelope
			if err := json.Unmarshal(msg, &env); err == nil {
				out = append(out, env)
			}
		default:
			return out
		}
	}
}

func TestSubscriptionBookkeeping(t *testing.T) {
	h := New(zap.NewNop())
	c := testClient()

	h.ApplySubscription(c, []int64{100, 200}, true)
	if !c.IsSubscribed(100) || !c.IsSubscribed(200) {
		t.Error("both items should be subscribed")
	}
	if c.IsSubscribed(300) {
		t.Error("an unsubscribed item must not report as subscribed")
	}

	h.ApplySubscription(c, []int64{100}, false)
	if c.IsSubscribed(100) {
		t.Error("item 100 should have been unsubscribed")
	}
	if !c.IsSubscribed(200) {
		t.Error("unsubscribing one item must not affect the others")
	}
}

// Price updates are the reason a client holds the socket open; sending
// them to unsubscribed clients would flood every frontend with the whole
// catalogue.
func TestBroadcastPriceOnlyReachesSubscribers(t *testing.T) {
	h := New(zap.NewNop())
	subscriber, bystander := testClient(), testClient()

	h.ApplySubscription(subscriber, []int64{100}, true)
	h.ApplySubscription(bystander, []int64{999}, true)

	h.BroadcastPrice(100, []byte(`{"item_id":100,"price":1000}`))

	got := drain(subscriber)
	if len(got) != 1 {
		t.Fatalf("subscriber got %d frames, want 1", len(got))
	}
	if got[0].Type != EnvelopePrice {
		t.Errorf("envelope type = %q, want %q", got[0].Type, EnvelopePrice)
	}
	if len(drain(bystander)) != 0 {
		t.Error("a client subscribed to another item must receive nothing")
	}
}

func TestBroadcastPriceToNobodyIsHarmless(t *testing.T) {
	h := New(zap.NewNop())
	h.BroadcastPrice(100, []byte(`{}`))
}

// The heartbeat used to be sent as a chat message, which made every
// frontend render a junk entry once a minute.
func TestHeartbeatIsNotAChatMessage(t *testing.T) {
	h := New(zap.NewNop())
	h.BroadcastHeartbeat([]byte(`{"ts":"2026-09-15T12:00:00Z"}`))

	select {
	case env := <-h.broadcast:
		if env.Type != EnvelopeHeartbeat {
			t.Errorf("type = %q, want %q", env.Type, EnvelopeHeartbeat)
		}
		if env.Type == EnvelopeChat {
			t.Error("a heartbeat must never be delivered as chat")
		}
	default:
		t.Fatal("expected a heartbeat on the broadcast channel")
	}
}

func TestBroadcastChatUsesChatEnvelope(t *testing.T) {
	h := New(zap.NewNop())
	h.BroadcastChat([]byte(`{"body":"hello"}`))

	select {
	case env := <-h.broadcast:
		if env.Type != EnvelopeChat {
			t.Errorf("type = %q, want %q", env.Type, EnvelopeChat)
		}
	default:
		t.Fatal("expected a chat frame on the broadcast channel")
	}
}

// A slow or wedged consumer must not stall the price subscriber, which
// feeds every connected client.
func TestBroadcastDoesNotBlockWhenBufferIsFull(t *testing.T) {
	h := New(zap.NewNop())
	for i := 0; i < cap(h.broadcast)+10; i++ {
		h.BroadcastChat([]byte(`{}`))
	}
}

func TestUnsubscribeUnknownItemIsHarmless(t *testing.T) {
	h := New(zap.NewNop())
	c := testClient()
	h.ApplySubscription(c, []int64{404}, false)

	if c.IsSubscribed(404) {
		t.Error("should not be subscribed")
	}
}
