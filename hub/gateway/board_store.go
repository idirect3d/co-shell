package gateway

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Board persistence buckets (FEATURE-530).
var (
	bucketAgents   = []byte("agents")    // agentID -> AgentInfo
	bucketMessages = []byte("messages")  // seq (BE u64) -> Message
	bucketConvs    = []byte("convs")     // convKey -> conversation
	bucketConvMsgs = []byte("conv_msgs") // convKey \x00 seq (BE u64) -> messageID
	bucketAwaiting = []byte("awaiting")  // messageID|agentID -> Awaiting
	bucketMeta     = []byte("meta")      // "seq" / "roster_rev"
)

// BoardStore persists board state to a local bbolt database so conversations,
// the roster revision and pending reply obligations survive a hub restart.
type BoardStore struct {
	db *bolt.DB
}

// boardState is the full persisted state loaded at startup.
type boardState struct {
	agents    []AgentInfo
	messages  []Message
	convs     []conversation
	awaiting  map[string]*Awaiting
	seq       int
	rosterRev int
}

// OpenBoardStore opens (creating if needed) the board database at path.
func OpenBoardStore(path string) (*BoardStore, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create board db dir: %w", err)
		}
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open board db %s: %w", path, err)
	}
	store := &BoardStore{db: db}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketAgents, bucketMessages, bucketConvs, bucketConvMsgs, bucketAwaiting, bucketMeta} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init board db: %w", err)
	}
	return store, nil
}

// Close closes the database.
func (s *BoardStore) Close() error { return s.db.Close() }

// Load reads the whole persisted state.
func (s *BoardStore) Load() (*boardState, error) {
	st := &boardState{awaiting: make(map[string]*Awaiting)}
	err := s.db.View(func(tx *bolt.Tx) error {
		if err := tx.Bucket(bucketAgents).ForEach(func(_, v []byte) error {
			var a AgentInfo
			if err := json.Unmarshal(v, &a); err != nil {
				return err
			}
			st.agents = append(st.agents, a)
			return nil
		}); err != nil {
			return err
		}
		if err := tx.Bucket(bucketMessages).ForEach(func(_, v []byte) error {
			var m Message
			if err := json.Unmarshal(v, &m); err != nil {
				return err
			}
			st.messages = append(st.messages, m)
			return nil
		}); err != nil {
			return err
		}
		if err := tx.Bucket(bucketConvs).ForEach(func(_, v []byte) error {
			var c conversation
			if err := json.Unmarshal(v, &c); err != nil {
				return err
			}
			st.convs = append(st.convs, c)
			return nil
		}); err != nil {
			return err
		}
		if err := tx.Bucket(bucketAwaiting).ForEach(func(k, v []byte) error {
			var a Awaiting
			if err := json.Unmarshal(v, &a); err != nil {
				return err
			}
			st.awaiting[string(k)] = &a
			return nil
		}); err != nil {
			return err
		}
		meta := tx.Bucket(bucketMeta)
		st.seq = readInt(meta.Get([]byte("seq")))
		st.rosterRev = readInt(meta.Get([]byte("roster_rev")))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load board state: %w", err)
	}
	return st, nil
}

// SaveAgent stores one roster entry.
func (s *BoardStore) SaveAgent(a AgentInfo) error {
	return s.put(bucketAgents, []byte(a.ID), a)
}

// SaveMessage stores one message keyed by its sequence number.
func (s *BoardStore) SaveMessage(m *Message) error {
	return s.put(bucketMessages, seqKey(nil, m.Seq), m)
}

// SaveConv stores one conversation index entry.
func (s *BoardStore) SaveConv(c conversation) error {
	return s.put(bucketConvs, []byte(c.Key), c)
}

// AppendConvMessage records the conversation -> message index entry.
func (s *BoardStore) AppendConvMessage(convKey string, seq int, id string) error {
	key := append([]byte(convKey+"\x00"), seqKey(nil, seq)...)
	return s.put(bucketConvMsgs, key, []byte(id))
}

// SaveAwaiting stores one pending reply obligation.
func (s *BoardStore) SaveAwaiting(a *Awaiting) error {
	return s.put(bucketAwaiting, []byte(awaitingKey(a.MessageID, a.AgentID)), a)
}

// SaveMeta stores the sequence counter and roster revision.
func (s *BoardStore) SaveMeta(seq, rev int) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		meta := tx.Bucket(bucketMeta)
		if err := meta.Put([]byte("seq"), intKey(seq)); err != nil {
			return err
		}
		return meta.Put([]byte("roster_rev"), intKey(rev))
	})
}

// put marshals v and stores it under key in the named bucket.
func (s *BoardStore) put(bucket, key []byte, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucket).Put(key, data)
	})
}

// seqKey appends an 8-byte big-endian encoding of seq to dst.
func seqKey(dst []byte, seq int) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(seq))
	return append(dst, buf[:]...)
}

// intKey encodes n as 8 bytes big-endian.
func intKey(n int) []byte {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(n))
	return buf[:]
}

// readInt decodes an 8-byte big-endian integer (0 when empty).
func readInt(data []byte) int {
	if len(data) != 8 {
		return 0
	}
	return int(binary.BigEndian.Uint64(data))
}

// marshalEnvelope builds a flat downlink frame: the payload fields plus "type".
// co-shell expects flat messages (no nested payload) on this channel.
func marshalEnvelope(msgType string, payload interface{}) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	fields := map[string]interface{}{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	fields["type"] = msgType
	return json.Marshal(fields)
}
