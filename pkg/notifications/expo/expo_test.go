package expo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockExpo spins up a mock Expo Push service that records the request body and
// returns a canned response. The send and receipts endpoints share the server
// and dispatch on the path.
func mockExpo(t *testing.T, sendResp, receiptResp any) (*Client, *[]string, *[]string) {
	t.Helper()
	var sendBodies, receiptBodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readBody(t, r)
		switch r.URL.Path {
		case "/--/api/v2/push/send":
			sendBodies = append(sendBodies, body)
			writeJSON(t, w, sendResp)
		case "/--/api/v2/push/getReceipts":
			receiptBodies = append(receiptBodies, body)
			writeJSON(t, w, receiptResp)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	c := NewClient(nil, "test-token")
	c.sendURL = srv.URL + "/--/api/v2/push/send"
	c.receiptURL = srv.URL + "/--/api/v2/push/getReceipts"
	return c, &sendBodies, &receiptBodies
}

func readBody(t *testing.T, r *http.Request) string {
	t.Helper()
	buf := make([]byte, r.ContentLength)
	if r.ContentLength > 0 {
		_, _ = r.Body.Read(buf)
	}
	return string(buf)
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(v))
}

func TestSend_HappyPath(t *testing.T) {
	canned := SendResponse{
		Data: []Ticket{
			{Status: "ok", ID: "ticket-1"},
			{Status: "ok", ID: "ticket-2"},
		},
	}
	c, sendBodies, _ := mockExpo(t, canned, nil)

	msgs := []PushMessage{
		{To: "ExponentPushToken[a1]", Title: "Reminder", Body: "Check in!", Data: map[string]any{"habitId": "h1"}, Sound: "default", Priority: "high"},
		{To: "ExponentPushToken[b2]", Title: "Weekly review", Body: "Ready"},
	}
	tickets, err := c.Send(context.Background(), msgs)
	require.NoError(t, err)
	require.Len(t, tickets, 2)
	assert.Equal(t, "ticket-1", tickets[0].ID)
	assert.Equal(t, "ticket-2", tickets[1].ID)

	// Verify the request body round-trips.
	require.Len(t, *sendBodies, 1)
	var sent []PushMessage
	require.NoError(t, json.Unmarshal([]byte((*sendBodies)[0]), &sent))
	require.Len(t, sent, 2)
	assert.Equal(t, "ExponentPushToken[a1]", sent[0].To)
	assert.Equal(t, "default", sent[0].Sound)
	assert.Equal(t, "high", sent[0].Priority)
	assert.Equal(t, "h1", sent[0].Data["habitId"])
}

func TestSendOne(t *testing.T) {
	canned := SendResponse{Data: []Ticket{{Status: "ok", ID: "t1"}}}
	c, _, _ := mockExpo(t, canned, nil)

	ticket, err := c.SendOne(context.Background(), PushMessage{To: "ExponentPushToken[x]", Title: "Hi"})
	require.NoError(t, err)
	assert.Equal(t, "ok", ticket.Status)
	assert.Equal(t, "t1", ticket.ID)
}

func TestSend_RejectsInvalidToken(t *testing.T) {
	c, _, _ := mockExpo(t, SendResponse{}, nil)
	_, err := c.Send(context.Background(), []PushMessage{{To: "not-a-token", Title: "x"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid token")
}

func TestSend_RejectsTooManyMessages(t *testing.T) {
	c, _, _ := mockExpo(t, SendResponse{}, nil)
	msgs := make([]PushMessage, 101)
	for i := range msgs {
		msgs[i] = PushMessage{To: "ExponentPushToken[x]"}
	}
	_, err := c.Send(context.Background(), msgs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most 100")
}

func TestSend_EmptyIsNoop(t *testing.T) {
	c, _, _ := mockExpo(t, SendResponse{}, nil)
	tickets, err := c.Send(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, tickets)
}

func TestSend_PropagatesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"errors":[{"code":"RATE_LIMITED"}]}`))
	}))
	t.Cleanup(srv.Close)
	c := NewClient(nil, "")
	c.sendURL = srv.URL + "/--/api/v2/push/send"

	_, err := c.Send(context.Background(), []PushMessage{{To: "ExponentPushToken[a]"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
}

func TestSend_TicketCountMismatch(t *testing.T) {
	// Server returns 1 ticket for a 2-message request.
	canned := SendResponse{Data: []Ticket{{Status: "ok", ID: "t1"}}}
	c, _, _ := mockExpo(t, canned, nil)

	_, err := c.Send(context.Background(), []PushMessage{
		{To: "ExponentPushToken[a]"},
		{To: "ExponentPushToken[b]"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

func TestSend_ErrorTicket(t *testing.T) {
	canned := SendResponse{Data: []Ticket{
		{Status: "error", Message: "InvalidCredentials", Details: map[string]string{"error": "InvalidCredentials"}},
	}}
	c, _, _ := mockExpo(t, canned, nil)

	tickets, err := c.Send(context.Background(), []PushMessage{{To: "ExponentPushToken[a]"}})
	require.NoError(t, err)
	require.Len(t, tickets, 1)
	assert.Equal(t, "error", tickets[0].Status)
	assert.Equal(t, "InvalidCredentials", tickets[0].Details["error"])
}

func TestCheckReceipts_HappyPath(t *testing.T) {
	canned := ReceiptsResponse{Data: map[string]Receipt{
		"t1": {Status: "ok"},
		"t2": {Status: "error", Message: "Device not registered", Details: map[string]string{"error": "DeviceNotRegistered"}},
	}}
	c, _, receiptBodies := mockExpo(t, nil, canned)

	receipts, err := c.CheckReceipts(context.Background(), []string{"t1", "t2"})
	require.NoError(t, err)
	assert.Len(t, receipts, 2)
	assert.Equal(t, "ok", receipts["t1"].Status)
	assert.True(t, IsDeviceNotRegistered(receipts["t2"]))

	// Verify the request body.
	require.Len(t, *receiptBodies, 1)
	var sent struct {
		IDs []string `json:"ids"`
	}
	require.NoError(t, json.Unmarshal([]byte((*receiptBodies)[0]), &sent))
	assert.Equal(t, []string{"t1", "t2"}, sent.IDs)
}

func TestCheckReceipts_EmptyIsNoop(t *testing.T) {
	c, _, _ := mockExpo(t, nil, ReceiptsResponse{})
	receipts, err := c.CheckReceipts(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, receipts)
}

func TestCheckReceipts_RejectsTooManyIDs(t *testing.T) {
	c, _, _ := mockExpo(t, nil, ReceiptsResponse{})
	ids := make([]string, 1001)
	for i := range ids {
		ids[i] = "t"
	}
	_, err := c.CheckReceipts(context.Background(), ids)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most 1000")
}

func TestIsDeviceNotRegistered(t *testing.T) {
	assert.True(t, IsDeviceNotRegistered(Receipt{Status: "error", Details: map[string]string{"error": "DeviceNotRegistered"}}))
	assert.False(t, IsDeviceNotRegistered(Receipt{Status: "ok"}))
	assert.False(t, IsDeviceNotRegistered(Receipt{Status: "error", Details: map[string]string{"error": "MessageRateExceeded"}}))
}

func TestSend_SetsRequiredHeaders(t *testing.T) {
	var seenHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHeaders = r.Header.Clone()
		writeJSON(t, w, SendResponse{Data: []Ticket{{Status: "ok", ID: "t1"}}})
	}))
	t.Cleanup(srv.Close)
	c := NewClient(nil, "my-token")
	c.sendURL = srv.URL + "/--/api/v2/push/send"

	_, err := c.Send(context.Background(), []PushMessage{{To: "ExponentPushToken[a]", Title: "x"}})
	require.NoError(t, err)
	assert.Equal(t, "application/json", seenHeaders.Get("Content-Type"))
	assert.Equal(t, "application/json", seenHeaders.Get("Accept"))
	assert.Equal(t, "Bearer my-token", seenHeaders.Get("Authorization"))
	// Accept-Encoding is set by the client per Expo docs.
	assert.Contains(t, seenHeaders.Get("Accept-Encoding"), "gzip")
}

func TestSend_AcceptsExpoPushTokenPrefix(t *testing.T) {
	// Expo also supports "ExpoPushToken[...]" (the newer prefix used by some
	// bare-workflow and EAS builds). The client must accept both.
	canned := SendResponse{Data: []Ticket{{Status: "ok", ID: "t1"}}}
	c, _, _ := mockExpo(t, canned, nil)

	_, err := c.Send(context.Background(), []PushMessage{{To: "ExpoPushToken[newformat]"}})
	require.NoError(t, err)
}

func TestSend_RejectsEmptyToken(t *testing.T) {
	c, _, _ := mockExpo(t, SendResponse{}, nil)
	_, err := c.Send(context.Background(), []PushMessage{{To: "", Title: "x"}})
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "invalid token")
}
