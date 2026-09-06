package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// Conn represents an authenticated client connection.
// It exposes a Send method so the business layer (step 4+) can push
// agent events back to the client.
type Conn struct {
	conn net.Conn
	mu   sync.Mutex
}

// RemoteAddr returns the remote network address of the client.
func (c *Conn) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

// Send writes a JSON envelope to the client. Safe for concurrent use.
func (c *Conn) Send(env *Envelope) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return writeFrame(c.conn, env)
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// Handler processes authenticated business messages from a client.
// The gateway does not interpret business payloads; it delegates them here.
// Implementations must not block indefinitely; return promptly.
type Handler interface {
	// HandleMessage is invoked for each authenticated non-control message.
	HandleMessage(ctx context.Context, c *Conn, env *Envelope)
	// OnConnect is invoked once a client has authenticated successfully.
	OnConnect(ctx context.Context, c *Conn)
	// OnDisconnect is invoked when a client connection ends.
	OnDisconnect(ctx context.Context, c *Conn)
}

// Server is the gateway TCP service with API Key authentication.
type Server struct {
	cfg     *Config
	auth    *Authenticator
	handler Handler
	ln      net.Listener
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewServer creates a gateway Server.
func NewServer(cfg *Config, handler Handler) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.MaxFrameSize <= 0 {
		cfg.MaxFrameSize = DefaultConfig().MaxFrameSize
	}
	if cfg.AuthTimeout <= 0 {
		cfg.AuthTimeout = DefaultConfig().AuthTimeout
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		cfg:     cfg,
		auth:    NewAuthenticator(cfg.APIKey),
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Listen binds the TCP listener. Call Serve to accept connections.
func (s *Server) Listen() error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.cfg.ListenAddr, err)
	}
	s.ln = ln
	log.Printf("gateway: TCP listener on %s", s.cfg.ListenAddr)
	return nil
}

// Addr returns the bound listener address (valid after Listen).
func (s *Server) Addr() net.Addr {
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

// Serve accepts connections until the server is closed.
func (s *Server) Serve() error {
	if s.ln == nil {
		return errors.New("gateway: Serve called before Listen")
	}
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				return nil // shutting down
			}
			log.Printf("gateway: accept error: %v", err)
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(conn)
		}()
	}
}

// Close stops the server and closes all accepted connections.
func (s *Server) Close() error {
	s.cancel()
	if s.ln != nil {
		s.ln.Close()
	}
	s.wg.Wait()
	return nil
}

// handleConn runs the per-connection lifecycle: auth handshake, then dispatch.
func (s *Server) handleConn(raw net.Conn) {
	remote := raw.RemoteAddr().String()
	defer raw.Close()

	// Enforce auth timeout.
	_ = raw.SetDeadline(time.Now().Add(time.Duration(s.cfg.AuthTimeout) * time.Second))

	// Read the auth request.
	var authEnv Envelope
	if err := readFrame(raw, s.cfg.MaxFrameSize, &authEnv); err != nil {
		log.Printf("gateway: auth read error from %s: %v", remote, err)
		return
	}
	if authEnv.Type != MsgTypeAuth {
		_ = writeFrame(raw, &Envelope{Type: MsgTypeError, Payload: json.RawMessage(`"auth required"`)})
		log.Printf("gateway: %s sent %q before auth", remote, authEnv.Type)
		return
	}
	var req AuthRequest
	if err := json.Unmarshal(authEnv.Payload, &req); err != nil {
		_ = writeFrame(raw, &Envelope{Type: MsgTypeError, Payload: json.RawMessage(`"bad auth payload"`)})
		return
	}
	if !s.auth.Authenticate(req.APIKey) {
		_ = writeFrame(raw, &Envelope{Type: MsgTypeError, Payload: json.RawMessage(`"invalid api key"`)})
		log.Printf("gateway: auth failed from %s", remote)
		return
	}

	// Auth OK: clear deadline and notify.
	_ = raw.SetDeadline(time.Time{})
	_ = writeFrame(raw, &Envelope{Type: MsgTypeAuthAck})

	c := &Conn{conn: raw}
	if s.handler != nil {
		s.handler.OnConnect(s.ctx, c)
		defer s.handler.OnDisconnect(s.ctx, c)
	}

	// Read loop for business messages.
	for {
		var env Envelope
		if err := readFrame(raw, s.cfg.MaxFrameSize, &env); err != nil {
			if err != io.EOF && s.ctx.Err() == nil {
				log.Printf("gateway: read error from %s: %v", remote, err)
			}
			return
		}
		switch env.Type {
		case MsgTypePing:
			_ = c.Send(&Envelope{Type: MsgTypePong})
		case MsgTypePong:
			// ignore
		default:
			if s.handler != nil {
				s.handler.HandleMessage(s.ctx, c, &env)
			}
		}
	}
}
