package iso8583

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

// MessageHandler is called for each inbound ISO 8583 message received from
// the SmartVista network. It receives the raw bytes and returns the response
// bytes to send back.
type MessageHandler func(ctx context.Context, data []byte) ([]byte, error)

// Router manages a persistent TCP connection to the SmartVista/EthSwitch
// card authorization host. It handles:
//   - Connection lifecycle with automatic reconnection
//   - Length-prefixed framing (2-byte big-endian header)
//   - Inbound message dispatch to a registered handler
//   - Outbound message sending with correlation
type Router struct {
	addr    string
	handler MessageHandler
	codec   *Codec
	log     *slog.Logger

	conn    net.Conn
	mu      sync.Mutex
	done    chan struct{}

	// pending tracks outbound messages awaiting responses, keyed by STAN.
	pending   map[string]chan []byte
	pendingMu sync.Mutex
}

// NewRouter creates a new TCP socket router for ISO 8583 message exchange.
func NewRouter(addr string, handler MessageHandler, log *slog.Logger) *Router {
	return &Router{
		addr:    addr,
		handler: handler,
		codec:   NewCodec(),
		log:     log,
		done:    make(chan struct{}),
		pending: make(map[string]chan []byte),
	}
}

// Start establishes the TCP connection and begins listening for messages.
// It blocks until the context is cancelled or an unrecoverable error occurs.
func (r *Router) Start(ctx context.Context) error {
	if err := r.connect(); err != nil {
		return fmt.Errorf("initial connection to %s: %w", r.addr, err)
	}

	go r.readLoop(ctx)
	go r.keepAlive(ctx)

	<-ctx.Done()
	close(r.done)
	r.mu.Lock()
	if r.conn != nil {
		r.conn.Close()
	}
	r.mu.Unlock()
	return nil
}

// SendAndWait sends an outbound authorization request and blocks until
// the matching response arrives (correlated by STAN) or the timeout expires.
func (r *Router) SendAndWait(ctx context.Context, req *AuthorizationRequest, timeout time.Duration) (*AuthorizationResponse, error) {
	data, err := r.codec.PackAuthRequest(req)
	if err != nil {
		return nil, fmt.Errorf("packing auth request: %w", err)
	}

	// Register a response channel keyed by STAN
	respCh := make(chan []byte, 1)
	r.pendingMu.Lock()
	r.pending[req.STAN] = respCh
	r.pendingMu.Unlock()

	defer func() {
		r.pendingMu.Lock()
		delete(r.pending, req.STAN)
		r.pendingMu.Unlock()
	}()

	if err := r.send(data); err != nil {
		return nil, fmt.Errorf("sending auth request: %w", err)
	}

	select {
	case respData := <-respCh:
		return r.codec.UnpackAuthResponse(respData)
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for auth response (STAN=%s)", req.STAN)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *Router) connect() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn, err := net.DialTimeout("tcp", r.addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("dialing %s: %w", r.addr, err)
	}
	r.conn = conn
	r.log.Info("connected to SmartVista", slog.String("addr", r.addr))
	return nil
}

func (r *Router) reconnect() {
	r.mu.Lock()
	if r.conn != nil {
		r.conn.Close()
	}
	r.mu.Unlock()

	backoff := time.Second
	for {
		select {
		case <-r.done:
			return
		default:
		}

		r.log.Info("attempting reconnection", slog.String("addr", r.addr), slog.Duration("backoff", backoff))
		if err := r.connect(); err != nil {
			r.log.Error("reconnection failed", slog.String("error", err.Error()))
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		return
	}
}

// readLoop continuously reads length-prefixed messages from the socket.
func (r *Router) readLoop(ctx context.Context) {
	for {
		select {
		case <-r.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		data, err := r.readFrame()
		if err != nil {
			if err == io.EOF {
				r.log.Warn("connection closed by remote, reconnecting...")
			} else {
				r.log.Error("read error", slog.String("error", err.Error()))
			}
			r.reconnect()
			continue
		}

		go r.dispatch(ctx, data)
	}
}

// dispatch routes an inbound message: if it's a response (0110/0210), it
// correlates with a pending outbound; otherwise delegates to the handler.
func (r *Router) dispatch(ctx context.Context, data []byte) {
	if len(data) < 4 {
		r.log.Warn("received short message, ignoring", slog.Int("len", len(data)))
		return
	}

	mti := string(data[:4])
	switch mti {
	case "0110", "0210", "0410", "0430":
		// Response message -- correlate by STAN
		resp, err := r.codec.UnpackAuthResponse(data)
		if err != nil {
			r.log.Error("failed to unpack response", slog.String("error", err.Error()))
			return
		}
		r.pendingMu.Lock()
		ch, ok := r.pending[resp.STAN]
		r.pendingMu.Unlock()
		if ok {
			ch <- data
		} else {
			r.log.Warn("received response for unknown STAN", slog.String("stan", resp.STAN), slog.String("mti", mti))
		}

	case "0100", "0200", "0400", "0420":
		// Inbound request -- delegate to handler
		if r.handler == nil {
			r.log.Warn("no handler registered, dropping message", slog.String("mti", mti))
			return
		}
		respData, err := r.handler(ctx, data)
		if err != nil {
			r.log.Error("handler error", slog.String("mti", mti), slog.String("error", err.Error()))
			return
		}
		if respData != nil {
			if err := r.send(respData); err != nil {
				r.log.Error("failed to send response", slog.String("error", err.Error()))
			}
		}

	default:
		r.log.Warn("unrecognized MTI", slog.String("mti", mti))
	}
}

// readFrame reads a single length-prefixed ISO 8583 message.
// Wire format: [2-byte big-endian length][message payload]
func (r *Router) readFrame() ([]byte, error) {
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("no active connection")
	}

	lengthBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, lengthBuf); err != nil {
		return nil, err
	}
	msgLen := binary.BigEndian.Uint16(lengthBuf)

	if msgLen == 0 || msgLen > 9999 {
		return nil, fmt.Errorf("invalid message length: %d", msgLen)
	}

	payload := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, fmt.Errorf("reading message payload: %w", err)
	}

	return payload, nil
}

// send writes a length-prefixed message to the socket.
func (r *Router) send(data []byte) error {
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("no active connection")
	}

	frame := make([]byte, 2+len(data))
	binary.BigEndian.PutUint16(frame[:2], uint16(len(data)))
	copy(frame[2:], data)

	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("setting write deadline: %w", err)
	}

	_, err := conn.Write(frame)
	return err
}

// keepAlive sends periodic echo messages to prevent connection timeout.
func (r *Router) keepAlive(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Network management message (0800 echo)
			echoReq := &AuthorizationRequest{
				MTI:  "0800",
				STAN: "000000",
			}
			data, err := r.codec.PackAuthRequest(echoReq)
			if err != nil {
				r.log.Error("failed to pack keepalive", slog.String("error", err.Error()))
				continue
			}
			if err := r.send(data); err != nil {
				r.log.Warn("keepalive send failed, triggering reconnect", slog.String("error", err.Error()))
				r.reconnect()
			}
		}
	}
}
